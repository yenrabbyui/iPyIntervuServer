package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var (
	ratingValueAlt    = `Not Ready Yet|Not Yet Ready|Weak|Competent|Looking Good|Strong|Good|Solid|Exceptional|Looks Great|Excellent|Outstanding|N/A|na`
	proseRatingLabels = ratingValueAlt

	bucketPatterns = map[string]*regexp.Regexp{
		"conceptualAssessmentBucket": regexp.MustCompile(`(?i)conceptualAssessmentBucket\s*[=:]\s*(` + ratingValueAlt + `)`),
		"codeAssessmentBucket":       regexp.MustCompile(`(?i)codeAssessmentBucket\s*[=:]\s*(` + ratingValueAlt + `)`),
		"bugAssessmentBucket":        regexp.MustCompile(`(?i)bugAssessmentBucket\s*[=:]\s*(` + ratingValueAlt + `)`),
		"businessDomain":             regexp.MustCompile(`(?i)businessDomain\s*[=:]\s*(.+)`),
	}
	jsonBucketPatterns = map[string]*regexp.Regexp{
		"conceptualAssessmentBucket": regexp.MustCompile(`(?i)"conceptualAssessmentBucket"\s*:\s*"(` + ratingValueAlt + `)"`),
		"codeAssessmentBucket":       regexp.MustCompile(`(?i)"codeAssessmentBucket"\s*:\s*"(` + ratingValueAlt + `)"`),
		"bugAssessmentBucket":        regexp.MustCompile(`(?i)"bugAssessmentBucket"\s*:\s*"(` + ratingValueAlt + `)"`),
	}
	ipyintervuBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipy(?:intervu)?\\s*\\n(.*?)\\n```")
	partialIPyIntervuOpenPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipy(?:intervu)?\\s*\\n([\\s\\S]+)$")
	proseConceptualBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)conceptual\s+(?:understanding\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(` + proseRatingLabels + `)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+conceptual\s+(?:understanding\s+)?(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(` + proseRatingLabels + `)`),
		regexp.MustCompile(`(?i)conceptual[^.\n]{0,80}\b(` + proseRatingLabels + `)\b`),
	}
	proseCodeBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)code\s+(?:problem\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(` + proseRatingLabels + `|N/A|na)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+code\s+(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(` + proseRatingLabels + `|N/A|na)`),
		regexp.MustCompile(`(?i)code[^.\n]{0,80}\b(` + proseRatingLabels + `|N/A|na)\b`),
	}
	proseBugBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)bug\s+(?:hunting\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(` + proseRatingLabels + `|N/A|na)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+bug\s+(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(` + proseRatingLabels + `|N/A|na)`),
		regexp.MustCompile(`(?i)bug[^.\n]{0,80}\b(` + proseRatingLabels + `|N/A|na)\b`),
	}
	modeWrapUpPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(let'?s|we'?ll|we will)\b[^\n.]{0,50}\b(move|proceed|continue|turn|shift)\b[^\n.]{0,50}\b(to|into|on to)\b[^\n.]{0,30}\b(code|coding|bug|debugging|results|assessment results)\b`),
		regexp.MustCompile(`(?i)\bready for (the )?(coding|code|bug|debugging)\b`),
		regexp.MustCompile(`(?i)\b(that (concludes|completes|covers|wraps up)|we('ve| have) covered)\b[^\n.]{0,60}\b(conceptual|code|coding|bug|debugging)\b`),
		regexp.MustCompile(`(?i)\bwrap(ping)? up\b[^\n.]{0,40}\b(conceptual|code|coding|bug|debugging)\b`),
		regexp.MustCompile(`(?i)\bno (more|further) (conceptual |code )?(interview )?questions\b`),
	}
	prematureCodeToBugPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(to|into|on to)\b[^\n.]{0,30}\b(bug|debugging)\b`),
		regexp.MustCompile(`(?i)\bready for (the )?(bug|debugging)\b`),
		regexp.MustCompile(`(?i)\b(that (concludes|completes|covers|wraps up)|we('ve| have) covered)\b[^\n.]{0,60}\b(code|coding)\b`),
		regexp.MustCompile(`(?i)\bwrap(ping)? up\b[^\n.]{0,40}\b(code|coding)\b`),
	}
	prematureBugToResultsPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(to|into|on to)\b[^\n.]{0,30}\b(results|assessment results)\b`),
	}
)

type ipyintervuTail struct {
	ConceptualAssessmentBucket string `json:"conceptualAssessmentBucket"`
	ConceptualAssessmentPhase  string `json:"conceptualAssessmentPhase"`
	CodeAssessmentBucket       string `json:"codeAssessmentBucket"`
	CodeAssessmentPhase        string `json:"codeAssessmentPhase"`
	BugAssessmentBucket        string `json:"bugAssessmentBucket"`
	BugAssessmentPhase         string `json:"bugAssessmentPhase"`
	BusinessDomain             any    `json:"businessDomain"`
	ActiveMode                 string `json:"activeMode"`
}

var jsonPhasePatterns = map[string]*regexp.Regexp{
	"conceptualAssessmentPhase": regexp.MustCompile(`(?i)"conceptualAssessmentPhase"\s*:\s*"(in[_ -]?progress|complete)"`),
	"codeAssessmentPhase":       regexp.MustCompile(`(?i)"codeAssessmentPhase"\s*:\s*"(in[_ -]?progress|complete)"`),
	"bugAssessmentPhase":        regexp.MustCompile(`(?i)"bugAssessmentPhase"\s*:\s*"(in[_ -]?progress|complete)"`),
}

func truncateSummary(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func applyPreChatUserUpdate(state *AgentSessionState, userMessage string) {
	state.MessageIndex++
	state.LastUserMessageRaw = userMessage
	state.FirstUserMessageSeen = true

	switch state.ConversationPhase {
	case phaseAwaitingMajor:
		if state.StudentMajor == "" && !isGreetingMessage(userMessage) && strings.TrimSpace(userMessage) != "" {
			state.StudentMajor = strings.TrimSpace(userMessage)
			state.ConversationPhase = phaseAwaitingKeyConcept
			state.PendingQuestion = "weeklyKeyConceptSelection"
			state.WaitingForUserResponse = true
		}
	case phaseAwaitingKeyConcept:
		if sel, ok := matchWeekSelection(userMessage); ok {
			state.SelectedKeyConcept = sel.SelectedKeyConcept
			state.CurrentWeekNumber = sel.CurrentWeekNumber
			state.ConversationPhase = phaseAssessmentInProgress
			state.ActiveMode = modeConceptual
			state.ModesCompleted = nil
			state.ConceptualAssessmentBucket = ""
			state.ConceptualAssessmentPhase = ""
			state.CodeAssessmentBucket = ""
			state.CodeAssessmentPhase = ""
			state.BugAssessmentBucket = ""
			state.BugAssessmentPhase = ""
			state.PendingQuestion = "conceptualAssessment"
			state.WaitingForUserResponse = true
			state.CoachingRequested = false
			state.CoachingEnteredBeforeResults = false
			markAssessmentStarted(state)
		}
	case phaseAssessmentInProgress:
		if isCoachingRequest(userMessage) {
			state.CoachingRequested = true
			state.ActiveMode = modeCoaching
			state.CoachingEnteredBeforeResults = true
		} else {
			if state.ActiveMode == modeCoaching {
				if resumed := resumeAssessmentActiveMode(state); resumed != "" {
					state.ActiveMode = resumed
					state.PendingQuestion = pendingQuestionForMode(resumed)
				}
			}
			clampAssessmentModeForward(state)
		}
	case phaseAssessmentResults:
		if isCoachingRequest(userMessage) {
			state.CoachingRequested = true
			state.ActiveMode = modeCoaching
		}
	}
}

func applyBootstrapState(state *AgentSessionState, assistant string) {
	state.StartupPromptShown = true
	state.FirstUserMessageSeen = true
	state.ConversationPhase = phaseAwaitingMajor
	state.WaitingForUserResponse = true
	state.PendingQuestion = "studentMajor"
	state.MessageIndex = 1
	state.LastAssistantSummary = truncateSummary(assistant, 500)
}

func parseAssistantStateSync(state *AgentSessionState, assistant string) {
	if block := ipyintervuBlockPattern.FindStringSubmatch(assistant); len(block) == 2 {
		applyIPyIntervuTailJSON(state, block[1])
	} else if partial := partialIPyIntervuOpenPattern.FindStringSubmatch(assistant); len(partial) == 2 {
		applyIPyIntervuTailJSON(state, partial[1])
	}

	for field, pattern := range bucketPatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		switch field {
		case "conceptualAssessmentBucket":
			applyLegacyBucketSync(state, modeConceptual, value)
		case "codeAssessmentBucket":
			applyLegacyBucketSync(state, modeCode, value)
		case "bugAssessmentBucket":
			applyLegacyBucketSync(state, modeBug, value)
		case "businessDomain":
			state.BusinessDomain = value
		}
	}

	for field, pattern := range jsonBucketPatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		switch field {
		case "conceptualAssessmentBucket":
			applyLegacyBucketSync(state, modeConceptual, match[1])
		case "codeAssessmentBucket":
			applyLegacyBucketSync(state, modeCode, match[1])
		case "bugAssessmentBucket":
			applyLegacyBucketSync(state, modeBug, match[1])
		}
	}

	for field, pattern := range jsonPhasePatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		applyModePhaseFromTail(state, phaseFieldToMode(field), match[1])
	}

	detectProseAssessmentBuckets(state, assistant)
}

func applyIPyIntervuTailJSON(state *AgentSessionState, rawJSON string) {
	jsonText := strings.TrimSpace(rawJSON)
	jsonText = strings.TrimSuffix(jsonText, "```")
	jsonText = strings.TrimSpace(jsonText)
	if jsonText == "" {
		return
	}

	var tail ipyintervuTail
	if err := json.Unmarshal([]byte(jsonText), &tail); err == nil {
		applyModeSyncFromTail(state, modeConceptual, tail.ConceptualAssessmentPhase, tail.ConceptualAssessmentBucket)
		applyModeSyncFromTail(state, modeCode, tail.CodeAssessmentPhase, tail.CodeAssessmentBucket)
		applyModeSyncFromTail(state, modeBug, tail.BugAssessmentPhase, tail.BugAssessmentBucket)
		if tail.BusinessDomain != nil {
			if raw, err := json.Marshal(tail.BusinessDomain); err == nil {
				state.BusinessDomain = string(raw)
			}
		}
		return
	}

	for field, pattern := range jsonPhasePatterns {
		match := pattern.FindStringSubmatch(jsonText)
		if len(match) < 2 {
			continue
		}
		applyModePhaseFromTail(state, phaseFieldToMode(field), normalizeAssessmentPhase(match[1]))
	}

	for field, pattern := range jsonBucketPatterns {
		match := pattern.FindStringSubmatch(jsonText)
		if len(match) < 2 {
			continue
		}
		applyLegacyBucketSync(state, bucketFieldToMode(field), match[1])
	}
}

func phaseFieldToMode(field string) string {
	switch field {
	case "conceptualAssessmentPhase":
		return modeConceptual
	case "codeAssessmentPhase":
		return modeCode
	case "bugAssessmentPhase":
		return modeBug
	default:
		return ""
	}
}

func bucketFieldToMode(field string) string {
	switch field {
	case "conceptualAssessmentBucket":
		return modeConceptual
	case "codeAssessmentBucket":
		return modeCode
	case "bugAssessmentBucket":
		return modeBug
	default:
		return ""
	}
}

func normalizeAssessmentPhase(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "in progress", "in-progress", "in_progress", "assessing", "ongoing":
		return assessmentPhaseInProgress
	case "complete", "completed", "done", "finished":
		return assessmentPhaseComplete
	default:
		return strings.TrimSpace(value)
	}
}

func isValidFinalBucket(bucket string) bool {
	switch bucket {
	case bucketNotReady, bucketCompetent, bucketExceptional, bucketNA:
		return true
	default:
		return false
	}
}

func modePhaseValue(state *AgentSessionState, mode string) string {
	switch mode {
	case modeConceptual:
		return state.ConceptualAssessmentPhase
	case modeCode:
		return state.CodeAssessmentPhase
	case modeBug:
		return state.BugAssessmentPhase
	default:
		return ""
	}
}

func setModePhase(state *AgentSessionState, mode, phase string) {
	switch mode {
	case modeConceptual:
		state.ConceptualAssessmentPhase = phase
	case modeCode:
		state.CodeAssessmentPhase = phase
	case modeBug:
		state.BugAssessmentPhase = phase
	}
}

func setModeBucket(state *AgentSessionState, mode, bucket string) {
	switch mode {
	case modeConceptual:
		state.ConceptualAssessmentBucket = bucket
	case modeCode:
		state.CodeAssessmentBucket = bucket
	case modeBug:
		state.BugAssessmentBucket = bucket
	}
}

func applyModePhaseFromTail(state *AgentSessionState, mode, phase string) {
	if mode == "" || phase == "" {
		return
	}
	if phase == assessmentPhaseInProgress && modeIsCompleted(state, mode) {
		return
	}
	setModePhase(state, mode, phase)
}

func applyModeSyncFromTail(state *AgentSessionState, mode, phase, bucket string) {
	if mode == "" {
		return
	}
	if phase == assessmentPhaseInProgress && modeIsCompleted(state, mode) {
		return
	}
	phase = normalizeAssessmentPhase(phase)
	bucket = normalizeAssessmentBucket(bucket)

	if phase != "" {
		setModePhase(state, mode, phase)
	}

	switch phase {
	case assessmentPhaseInProgress:
		return
	case assessmentPhaseComplete:
		if bucket != "" && isValidFinalBucket(bucket) {
			setModeBucket(state, mode, bucket)
		}
	default:
		if bucket != "" && isValidFinalBucket(bucket) {
			setModeBucket(state, mode, bucket)
			setModePhase(state, mode, assessmentPhaseComplete)
		}
	}
}

func applyLegacyBucketSync(state *AgentSessionState, mode, rawBucket string) {
	bucket := normalizeAssessmentBucket(rawBucket)
	if bucket == "" || !isValidFinalBucket(bucket) {
		return
	}
	if modePhaseValue(state, mode) == assessmentPhaseInProgress {
		return
	}
	setModeBucket(state, mode, bucket)
	setModePhase(state, mode, assessmentPhaseComplete)
}

func isConceptualModeComplete(state *AgentSessionState) bool {
	if state.ConceptualAssessmentBucket == "" || !isValidFinalBucket(state.ConceptualAssessmentBucket) {
		return false
	}
	if state.ConceptualAssessmentPhase == assessmentPhaseInProgress {
		return false
	}
	return state.ConceptualAssessmentPhase == assessmentPhaseComplete || state.ConceptualAssessmentPhase == ""
}

func isCodeModeComplete(state *AgentSessionState) bool {
	if state.CodeAssessmentBucket == "" || !isValidFinalBucket(state.CodeAssessmentBucket) {
		return false
	}
	if state.CodeAssessmentPhase == assessmentPhaseInProgress {
		return false
	}
	return state.CodeAssessmentPhase == assessmentPhaseComplete || state.CodeAssessmentPhase == ""
}

func isBugModeComplete(state *AgentSessionState) bool {
	if state.BugAssessmentBucket == "" || !isValidFinalBucket(state.BugAssessmentBucket) {
		return false
	}
	if state.BugAssessmentPhase == assessmentPhaseInProgress {
		return false
	}
	return state.BugAssessmentPhase == assessmentPhaseComplete || state.BugAssessmentPhase == ""
}

func normalizeAssessmentBucket(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "not yet ready", "not ready yet", "weak":
		return bucketNotReady
	case "looking good", "competent", "strong", "good", "solid":
		return bucketCompetent
	case "looks great", "exceptional", "excellent", "outstanding":
		return bucketExceptional
	case "n/a", "na":
		return bucketNA
	default:
		return strings.TrimSpace(value)
	}
}

func firstProseBucketMatch(assistant string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) >= 2 {
			return normalizeAssessmentBucket(match[1])
		}
	}
	return ""
}

func detectProseAssessmentBuckets(state *AgentSessionState, assistant string) {
	if state.ConceptualAssessmentPhase != assessmentPhaseInProgress && state.ConceptualAssessmentBucket == "" && state.ActiveMode == modeConceptual {
		if bucket := firstProseBucketMatch(assistant, proseConceptualBucketPatterns); bucket != "" {
			normalized := normalizeAssessmentBucket(bucket)
			if isValidFinalBucket(normalized) {
				state.ConceptualAssessmentBucket = normalized
				state.ConceptualAssessmentPhase = assessmentPhaseComplete
			}
		}
	}
	if state.CodeAssessmentPhase != assessmentPhaseInProgress && state.CodeAssessmentBucket == "" && state.ActiveMode == modeCode {
		if bucket := firstProseBucketMatch(assistant, proseCodeBucketPatterns); bucket != "" {
			normalized := normalizeAssessmentBucket(bucket)
			if isValidFinalBucket(normalized) {
				state.CodeAssessmentBucket = normalized
				state.CodeAssessmentPhase = assessmentPhaseComplete
			}
		}
	}
	if state.BugAssessmentPhase != assessmentPhaseInProgress && state.BugAssessmentBucket == "" && state.ActiveMode == modeBug {
		if bucket := firstProseBucketMatch(assistant, proseBugBucketPatterns); bucket != "" {
			normalized := normalizeAssessmentBucket(bucket)
			if isValidFinalBucket(normalized) {
				state.BugAssessmentBucket = normalized
				state.BugAssessmentPhase = assessmentPhaseComplete
			}
		}
	}
}

func appendModeCompleted(state *AgentSessionState, mode string) {
	for _, existing := range state.ModesCompleted {
		if existing == mode {
			return
		}
	}
	state.ModesCompleted = append(state.ModesCompleted, mode)
}

func computeFinalRating(state *AgentSessionState) string {
	if state.isProblemDecompositionWeek() {
		if state.ConceptualAssessmentBucket != "" {
			return state.ConceptualAssessmentBucket
		}
		return ""
	}

	buckets := []string{
		state.ConceptualAssessmentBucket,
		state.CodeAssessmentBucket,
		state.BugAssessmentBucket,
	}
	for _, bucket := range buckets {
		if bucket == bucketNotReady {
			return bucketNotReady
		}
	}
	hasCompetent := false
	allExceptional := true
	for _, bucket := range buckets {
		if bucket == "" || bucket == bucketNA {
			continue
		}
		if bucket == bucketCompetent {
			hasCompetent = true
			allExceptional = false
		}
		if bucket != bucketExceptional {
			allExceptional = false
		}
	}
	if hasCompetent {
		return bucketCompetent
	}
	if allExceptional {
		return bucketExceptional
	}
	return ""
}

func markAssessmentStarted(state *AgentSessionState) {
	now := time.Now()
	state.AssessmentStartTime = &now
	state.AssessmentEndTime = nil
	state.AssessmentComplete = false
}

func markAssessmentEnded(state *AgentSessionState) {
	if state.AssessmentEndTime != nil {
		return
	}
	now := time.Now()
	state.AssessmentEndTime = &now
}

func assessmentModeRank(mode string) int {
	switch mode {
	case modeConceptual:
		return 1
	case modeCode:
		return 2
	case modeBug:
		return 3
	default:
		return 0
	}
}

func modeIsCompleted(state *AgentSessionState, mode string) bool {
	for _, existing := range state.ModesCompleted {
		if existing == mode {
			return true
		}
	}
	switch mode {
	case modeConceptual:
		return isConceptualModeComplete(state)
	case modeCode:
		return isCodeModeComplete(state)
	case modeBug:
		return isBugModeComplete(state)
	default:
		return false
	}
}

func minimumAllowedAssessmentMode(state *AgentSessionState) string {
	if state.isProblemDecompositionWeek() {
		if !modeIsCompleted(state, modeConceptual) {
			return modeConceptual
		}
		return ""
	}
	if !modeIsCompleted(state, modeConceptual) {
		return modeConceptual
	}
	if !modeIsCompleted(state, modeCode) {
		return modeCode
	}
	if !modeIsCompleted(state, modeBug) {
		return modeBug
	}
	return ""
}

func resumeAssessmentActiveMode(state *AgentSessionState) string {
	return minimumAllowedAssessmentMode(state)
}

func pendingQuestionForMode(mode string) string {
	switch mode {
	case modeConceptual:
		return "conceptualAssessment"
	case modeCode:
		return "codeAssessment"
	case modeBug:
		return "bugAssessment"
	default:
		return ""
	}
}

// clampAssessmentModeForward enforces Conceptual → Code → Bug with no backward steps.
func clampAssessmentModeForward(state *AgentSessionState) {
	if state.ConversationPhase != phaseAssessmentInProgress || state.ActiveMode == modeCoaching {
		return
	}
	minMode := minimumAllowedAssessmentMode(state)
	if minMode == "" {
		return
	}
	minRank := assessmentModeRank(minMode)
	currentRank := assessmentModeRank(state.ActiveMode)
	if currentRank == 0 || currentRank < minRank {
		state.ActiveMode = minMode
		state.PendingQuestion = pendingQuestionForMode(minMode)
	}
}

// applyAutomaticModeTransitions updates activeMode after bucket assignment.
// Returns true when the server should immediately continue into the next assessment mode.
func applyAutomaticModeTransitions(state *AgentSessionState) bool {
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return false
	}

	if state.ConversationPhase == phaseAssessmentResults {
		return false
	}

	if state.isProblemDecompositionWeek() {
		if isConceptualModeComplete(state) && state.ConversationPhase != phaseAssessmentResults {
			appendModeCompleted(state, modeConceptual)
			state.CodeAssessmentBucket = bucketNA
			state.BugAssessmentBucket = bucketNA
			state.ActiveMode = ""
			state.ConversationPhase = phaseAssessmentResults
			state.FinalRating = computeFinalRating(state)
			state.AssessmentComplete = state.FinalRating != ""
			state.PendingQuestion = "assessmentResults"
			state.WaitingForUserResponse = true
			markAssessmentEnded(state)
		}
		return false
	}

	switch state.ActiveMode {
	case modeConceptual:
		if isConceptualModeComplete(state) {
			appendModeCompleted(state, modeConceptual)
			state.ActiveMode = modeCode
			state.CodeAssessmentPhase = ""
			state.PendingQuestion = "codeAssessment"
			state.WaitingForUserResponse = true
			return true
		}
	case modeCode:
		if isCodeModeComplete(state) {
			appendModeCompleted(state, modeCode)
			state.ActiveMode = modeBug
			state.BugAssessmentPhase = ""
			state.PendingQuestion = "bugAssessment"
			state.WaitingForUserResponse = true
			return true
		}
	case modeBug:
		if isBugModeComplete(state) {
			appendModeCompleted(state, modeBug)
			state.ActiveMode = ""
			state.ConversationPhase = phaseAssessmentResults
			state.FinalRating = computeFinalRating(state)
			state.AssessmentComplete = state.FinalRating != ""
			state.PendingQuestion = "assessmentResults"
			state.WaitingForUserResponse = true
			markAssessmentEnded(state)
		}
	}
	return false
}

func modeContinuationUserMessage(state *AgentSessionState) string {
	switch state.ActiveMode {
	case modeCode:
		return "[System handoff: Server state activeMode is now CodeProblem. Begin the code assessment immediately: Taylor and Morgan introduce themselves at the company from businessDomain, then present the first step of the coding interview (decomposition before code). End your reply with ```_ipyintervu``` including \"codeAssessmentPhase\": \"in_progress\". Do not ask the student to choose or confirm the next mode.]"
	case modeBug:
		return "[System handoff: Server state activeMode is now BugHunting. Begin the bug-hunting assessment immediately: Riley and Casey introduce themselves, present one buggy snippet with intended behavior, and ask how the student would find the bug (debugging process only—no request for corrected code). End your reply with ```_ipyintervu``` including \"bugAssessmentPhase\": \"in_progress\". Do not ask the student to choose or confirm the next mode.]"
	default:
		return ""
	}
}

const maxChatInternalTurns = 6

type assistantTurnFollowUp struct {
	ContinueTurn bool
	Handoff      string
	Kind         string // mode, results, assessment_sync
}

func currentModeBucketValue(state *AgentSessionState) string {
	switch state.ActiveMode {
	case modeConceptual:
		return state.ConceptualAssessmentBucket
	case modeCode:
		return state.CodeAssessmentBucket
	case modeBug:
		return state.BugAssessmentBucket
	default:
		return ""
	}
}

func hasIPyIntervuBlock(assistant string) bool {
	return ipyintervuBlockPattern.MatchString(assistant) || partialIPyIntervuOpenPattern.MatchString(assistant)
}

func currentModeSyncFields(state *AgentSessionState) (phaseField, bucketField, modeLabel string) {
	switch state.ActiveMode {
	case modeConceptual:
		return "conceptualAssessmentPhase", "conceptualAssessmentBucket", "Conceptual Understanding"
	case modeCode:
		return "codeAssessmentPhase", "codeAssessmentBucket", "Code Problem"
	case modeBug:
		return "bugAssessmentPhase", "bugAssessmentBucket", "Bug Hunting"
	default:
		return "", "", ""
	}
}

func currentModePhaseValue(state *AgentSessionState) string {
	return modePhaseValue(state, state.ActiveMode)
}

func isActiveModeComplete(state *AgentSessionState) bool {
	switch state.ActiveMode {
	case modeConceptual:
		return isConceptualModeComplete(state)
	case modeCode:
		return isCodeModeComplete(state)
	case modeBug:
		return isBugModeComplete(state)
	default:
		return false
	}
}

func ipyBlockMissingCurrentBucket(state *AgentSessionState, assistant string) bool {
	if !hasIPyIntervuBlock(assistant) {
		return false
	}
	lower := strings.ToLower(assistant)
	if strings.Contains(lower, "assessmentbucket") && currentModeBucketValue(state) == "" {
		return true
	}
	if state.ActiveMode == modeConceptual && strings.Contains(lower, "businessdomain") && !strings.Contains(lower, "conceptualassessmentbucket") && !strings.Contains(lower, "conceptualassessmentphase") {
		return false
	}
	return false
}

func looksLikeModeWrapUp(assistant string) bool {
	for _, pattern := range modeWrapUpPatterns {
		if pattern.MatchString(assistant) {
			return true
		}
	}
	return false
}

func looksLikePrematureModeTransition(state *AgentSessionState, assistant string) bool {
	switch state.ActiveMode {
	case modeCode:
		for _, pattern := range prematureCodeToBugPatterns {
			if pattern.MatchString(assistant) {
				return true
			}
		}
		return false
	case modeBug:
		for _, pattern := range prematureBugToResultsPatterns {
			if pattern.MatchString(assistant) {
				return true
			}
		}
		return false
	default:
		return looksLikeModeWrapUp(assistant)
	}
}

func shouldForceAssessmentSync(state *AgentSessionState, assistant string) bool {
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return false
	}
	if ipyBlockMissingCurrentBucket(state, assistant) {
		return true
	}
	if currentModePhaseValue(state) == assessmentPhaseComplete && currentModeBucketValue(state) == "" {
		return true
	}
	return looksLikePrematureModeTransition(state, assistant)
}

func assessmentSyncRetryMessage(state *AgentSessionState, assistant string) string {
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return ""
	}
	if state.ConversationPhase != phaseAssessmentInProgress || state.ActiveMode == "" || isActiveModeComplete(state) {
		return ""
	}

	phaseField, bucketField, label := currentModeSyncFields(state)
	if phaseField == "" {
		return ""
	}

	if !hasIPyIntervuBlock(assistant) {
		return "[System: Missing _ipyintervu sync block. Every " + label + " reply must end with ```_ipyintervu``` JSON for the active mode. While asking interview questions: {\"" + phaseField + "\": \"in_progress\"}. When finished with this mode (no more questions in this mode): {\"" + phaseField + "\": \"complete\", \"" + bucketField + "\": \"Not Ready Yet\"|\"Competent\"|\"Exceptional\"}. Use exact bucket strings. The server advances modes automatically; do not ask the user to choose the next mode.]"
	}

	if !shouldForceAssessmentSync(state, assistant) {
		return ""
	}

	return "[System: The " + label + " portion appears finished but the server did not receive assessmentPhase \"complete\" with " + bucketField + ". Reply with at most one brief neutral closing sentence (no new interview questions) and ```_ipyintervu``` JSON: {\"" + phaseField + "\": \"complete\", \"" + bucketField + "\": \"Not Ready Yet\"|\"Competent\"|\"Exceptional\"}. Use exact bucket strings—not Strong, Good, or other synonyms.]"
}

func assessmentResultsContinuationMessage(state *AgentSessionState) string {
	if state.ConversationPhase != phaseAssessmentResults || state.FinalRating == "" {
		return ""
	}
	return "[System handoff: Server state conversationPhase is now AssessmentResults. Present Assessment Results immediately using the exact conceptualAssessmentBucket, codeAssessmentBucket, bugAssessmentBucket, and finalRating strings from server state. Do not ask new interview questions or offer coaching unless the user explicitly requests it.]"
}

// postProcessAssistantTurn parses bucket sync, applies mode transitions, and decides
// whether the server should immediately call the model again (mode handoff, results, or bucket retry).
func postProcessAssistantTurn(state *AgentSessionState, assistant string, bucketSyncAttempted bool, syncLog *ipySyncLogContext) assistantTurnFollowUp {
	phaseBefore := state.ConversationPhase

	if syncLog != nil {
		complete, partial := collectIPyIntervuSyncPortions(assistant)
		logIPyIntervuSyncPortions(*syncLog, complete, partial)
	}

	parseAssistantStateSync(state, assistant)
	shouldContinueMode := applyAutomaticModeTransitions(state)
	clampAssessmentModeForward(state)
	state.LastAssistantSummary = truncateSummary(assistant, 500)

	if shouldContinueMode {
		if handoff := modeContinuationUserMessage(state); handoff != "" {
			return assistantTurnFollowUp{ContinueTurn: true, Handoff: handoff, Kind: "mode"}
		}
	}

	if state.ConversationPhase == phaseAssessmentResults && phaseBefore != phaseAssessmentResults {
		if handoff := assessmentResultsContinuationMessage(state); handoff != "" {
			return assistantTurnFollowUp{ContinueTurn: true, Handoff: handoff, Kind: "results"}
		}
	}

	if !bucketSyncAttempted && state.ConversationPhase == phaseAssessmentInProgress {
		if handoff := assessmentSyncRetryMessage(state, assistant); handoff != "" {
			return assistantTurnFollowUp{ContinueTurn: true, Handoff: handoff, Kind: "assessment_sync"}
		}
	}

	return assistantTurnFollowUp{}
}

func applyPostChatStateUpdate(state *AgentSessionState, userMessage, assistant string) bool {
	parseAssistantStateSync(state, assistant)
	shouldContinue := applyAutomaticModeTransitions(state)
	state.LastAssistantSummary = truncateSummary(assistant, 500)
	return shouldContinue
}
