package main

import (
	"hash/fnv"
	"math/rand/v2"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// chatEngine selects the /api/chat implementation: "d5" for the Interviewer + Evaluator
// engine (D5-interviewer-evaluator-design.md); anything else keeps the original engine.
var chatEngine = strings.ToLower(strings.TrimSpace(os.Getenv("IPY_ENGINE")))

func d5Enabled() bool {
	return chatEngine == "d5"
}

type d5Persona struct {
	Name string
	Role string
}

// d5PersonaRoles and d5NamePools are the canned personas (design §4.1): one interviewer
// per phase, its name drawn per session from the pool.
var d5PersonaRoles = map[string]string{
	modeConceptual: "hiring manager",
	modeCode:       "software developer",
	modeBug:        "QA engineer",
	modeCoaching:   "mentor",
}

var d5NamePools = map[string][]string{
	modeConceptual: {"Alex", "Jordan", "Priya", "Marcus", "Elena", "Sam"},
	modeCode:       {"Taylor", "Diego", "Mei", "Noah", "Aisha", "Chris"},
	modeBug:        {"Riley", "Omar", "Hannah", "Kenji", "Lucia", "Ben"},
	modeCoaching:   {"Samantha", "David", "Andre", "Nina", "Leo", "Farah"},
}

// d5PersonaNamePattern matches any pooled persona name, for the addressesPersona post-check.
var d5PersonaNamePattern = func() string {
	var names []string
	for _, pool := range d5NamePools {
		for _, n := range pool {
			names = append(names, strings.ToLower(n))
		}
	}
	return strings.Join(names, "|")
}()

func pickD5Personas(sessionID string) map[string]d5Persona {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sessionID))
	rng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), h.Sum64()))
	out := map[string]d5Persona{}
	for _, mode := range []string{modeConceptual, modeCode, modeBug, modeCoaching} {
		pool := d5NamePools[mode]
		out[mode] = d5Persona{Name: pool[rng.IntN(len(pool))], Role: d5PersonaRoles[mode]}
	}
	return out
}

type d5Message struct {
	Role    string // "assistant" (interviewer) or "user" (candidate)
	Content string
	Mode    string
}

// d5Session is the D5 engine's per-session state, hung off AgentSessionState. turnMu
// serializes chat turns for the session; mu guards the fields background Evaluator jobs
// write (brief, labels, evidence).
type d5Session struct {
	turnMu sync.Mutex
	mu     sync.Mutex

	Personas      map[string]d5Persona
	CompanyName   string
	CompanyDomain string

	Transcript []d5Message
	// Material is the scenario, task or snippet each mode opened with.
	Material map[string]string
	// NeedsOpening is set when a mode's opening could not be generated; the next student
	// message retries it.
	NeedsOpening bool

	AnswerIndex  int
	RedirectUsed map[string]bool
	// LastAskTarget is the rubric dimension the interviewer's latest question targets.
	LastAskTarget string
	LastMove      string
	// PendingIssues are post-check hits waiting for the next Evaluator run.
	PendingIssues []string

	Brief         *d5Brief
	briefInFlight chan struct{}
	ModeBriefs    map[string]*d5Brief
	Labels        map[string][]gradeLabel
	Evidence      map[string][]string
	VagueAnswers  map[int]bool
	labelJobs     sync.WaitGroup
	CodePasted    bool
}

func newD5Session(sessionID string) *d5Session {
	return &d5Session{
		Personas:     pickD5Personas(sessionID),
		Material:     map[string]string{},
		RedirectUsed: map[string]bool{},
		ModeBriefs:   map[string]*d5Brief{},
		Labels:       map[string][]gradeLabel{},
		Evidence:     map[string][]string{},
		VagueAnswers: map[int]bool{},
	}
}

func (s *d5Session) addLabels(mode string, labels []gradeLabel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range labels {
		if dimensionInMode(mode, l.Dimension) && levelRank(l.Level) >= 0 {
			s.Labels[mode] = append(s.Labels[mode], l)
		}
	}
}

func (s *d5Session) labelsSnapshot() map[string][]gradeLabel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]gradeLabel{}
	for mode, labels := range s.Labels {
		out[mode] = append([]gradeLabel(nil), labels...)
	}
	return out
}

func (s *d5Session) latestBrief() *d5Brief {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Brief
}

// modeMessages returns the transcript messages from one mode, most recent last.
func (s *d5Session) modeMessages(mode string) []d5Message {
	var out []d5Message
	for _, m := range s.Transcript {
		if m.Mode == mode {
			out = append(out, m)
		}
	}
	return out
}

// lastInterviewerMessage returns the interviewer's latest message in the transcript.
func (s *d5Session) lastInterviewerMessage() string {
	for i := len(s.Transcript) - 1; i >= 0; i-- {
		if s.Transcript[i].Role == "assistant" {
			return s.Transcript[i].Content
		}
	}
	return ""
}

func d5SessionFor(state *AgentSessionState, sessionID string) *d5Session {
	if state.D5 == nil {
		state.D5 = newD5Session(sessionID)
	}
	return state.D5
}

var articleVowelPattern = regexp.MustCompile(`(?i)^[aeiou]`)

func withArticle(phrase string) string {
	phrase = strings.TrimSpace(phrase)
	lower := strings.ToLower(phrase)
	if phrase == "" || strings.HasPrefix(lower, "a ") || strings.HasPrefix(lower, "an ") || strings.HasPrefix(lower, "the ") {
		return phrase
	}
	if articleVowelPattern.MatchString(phrase) {
		return "an " + phrase
	}
	return "a " + phrase
}

// personaIntro is the server-written introduction for a phase's interviewer (§4.1).
func (s *d5Session) personaIntro(mode string) string {
	p := s.Personas[mode]
	company := s.CompanyName
	if company == "" {
		return "Hi, I'm " + p.Name + ", " + withArticle(p.Role) + " on the team."
	}
	intro := "Hi, I'm " + p.Name + ", " + withArticle(p.Role) + " at " + company
	if s.CompanyDomain != "" {
		intro += ", " + withArticle(strings.TrimRight(s.CompanyDomain, "."))
	}
	return intro + "."
}
