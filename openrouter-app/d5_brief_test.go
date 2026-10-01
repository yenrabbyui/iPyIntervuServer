package main

import (
	"reflect"
	"testing"
)

func TestParseBriefDesignExample(t *testing.T) {
	text := `QUALITY: partial
CLARIFICATION: no
EVIDENCE: identified inputs ("the user types the order total"); described the discount step
COVERED: inputs; processing steps
GAPS: output format; invalid input handling
NEXT: Probe how they'd present the result to the customer.
FALLBACK: How would you show the final total to the customer?
AVOID: What inputs does the program need?
RECOMMEND: continue
LEVELS: decomposition=competent; correctness=competent; understanding=not_ready
ISSUES: Last question mentioned dictionaries (out of scope).`
	b, ok := parseBrief(text, modeCode, 4)
	if !ok {
		t.Fatal("brief not ok")
	}
	if b.Quality != "partial" || b.Clarification || b.Recommend != "continue" {
		t.Fatalf("enum fields wrong: %+v", b)
	}
	if !reflect.DeepEqual(b.Covered, []string{"inputs", "processing steps"}) || len(b.Evidence) != 2 {
		t.Fatalf("lists wrong: covered %v evidence %v", b.Covered, b.Evidence)
	}
	want := []gradeLabel{
		{dimDecomposition, levelCompetent, 4}, {dimCorrectness, levelCompetent, 4}, {dimUnderstanding, levelNotReady, 4},
	}
	if !reflect.DeepEqual(b.Levels, want) {
		t.Fatalf("levels = %+v", b.Levels)
	}
	if len(b.Issues) != 1 {
		t.Fatalf("issues = %v", b.Issues)
	}
}

func TestParseBriefTolerance(t *testing.T) {
	text := "Here are my notes:\n**NEXT:** Ask what happens with a negative score.\n- RECOMMEND: Close (enough evidence)\nLEVELS: conceptual = Not Yet Ready, ai_use=exceptional\nISSUES: none"
	b, ok := parseBrief(text, modeConceptual, 2)
	if !ok || b.Next != "Ask what happens with a negative score." {
		t.Fatalf("next = %q ok=%v", b.Next, ok)
	}
	if b.Recommend != "close" {
		t.Fatalf("recommend = %q", b.Recommend)
	}
	if len(b.Levels) != 1 || b.Levels[0].Dimension != dimConceptual || b.Levels[0].Level != levelNotReady {
		t.Fatalf("levels = %+v (ai_use must be dropped in Conceptual)", b.Levels)
	}
	if len(b.Issues) != 0 {
		t.Fatalf("issues = %v, want none", b.Issues)
	}
	if b.Quality != "" {
		t.Fatalf("missing quality should stay empty, got %q", b.Quality)
	}
}

func TestParseBriefFailsWithoutNextOrFallback(t *testing.T) {
	if _, ok := parseBrief("QUALITY: solid\nRECOMMEND: close", modeBug, 1); ok {
		t.Fatal("a brief without NEXT or FALLBACK must count as failed")
	}
}

func TestParseLevelsLine(t *testing.T) {
	got := parseLevelsLine("Sure.\nLEVELS: ai_use=competent; correctness=exceptional", modeCode, 7)
	want := []gradeLabel{{dimAIUse, levelCompetent, 7}, {dimCorrectness, levelExceptional, 7}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseCompanyLine(t *testing.T) {
	name, domain, rest, ok := parseCompanyLine("COMPANY: Brightline Bakery | small-batch bakery with online ordering.\nWe take pastry orders online.\nWhat would you do first?")
	if !ok || name != "Brightline Bakery" || domain != "small-batch bakery with online ordering" {
		t.Fatalf("got %q %q ok=%v", name, domain, ok)
	}
	if rest != "We take pastry orders online.\nWhat would you do first?" {
		t.Fatalf("rest = %q", rest)
	}
	if _, _, rest, ok := parseCompanyLine("We take pastry orders online. What first?"); ok || rest == "" {
		t.Fatalf("missing line: ok=%v rest=%q", ok, rest)
	}
	if name, _, _, ok := parseCompanyLine("**COMPANY:** Green Acres Farm - organic vegetable farm\nScenario."); !ok || name != "Green Acres Farm" {
		t.Fatalf("bold/dash form: name=%q ok=%v", name, ok)
	}
}

func TestPersonaIntro(t *testing.T) {
	s := newD5Session("s1")
	s.Personas[modeCode] = d5Persona{Name: "Mei", Role: "software developer"}
	s.CompanyName, s.CompanyDomain = "Brightline Bakery", "small-batch bakery with online ordering"
	want := "Hi, I'm Mei, a software developer at Brightline Bakery, a small-batch bakery with online ordering."
	if got := s.personaIntro(modeCode); got != want {
		t.Fatalf("got %q", got)
	}
	s.Personas[modeBug] = d5Persona{Name: "Omar", Role: "QA engineer"}
	s.CompanyDomain = "an organic farm"
	if got := s.personaIntro(modeBug); got != "Hi, I'm Omar, a QA engineer at Brightline Bakery, an organic farm." {
		t.Fatalf("got %q", got)
	}
}

func TestPickD5PersonasUsesPools(t *testing.T) {
	p := pickD5Personas("session-x")
	for mode, pool := range d5NamePools {
		found := false
		for _, n := range pool {
			found = found || n == p[mode].Name
		}
		if !found || p[mode].Role != d5PersonaRoles[mode] {
			t.Fatalf("mode %s persona %+v not from pool", mode, p[mode])
		}
	}
}
