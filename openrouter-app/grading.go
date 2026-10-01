package main

import "strings"

// Grading for the D5 engine, as specified in grading-rules.md. The model only labels
// answers; every rule that turns labels into buckets lives here.

const (
	levelNotReady    = "not_ready"
	levelCompetent   = "competent"
	levelExceptional = "exceptional"
)

const (
	dimConceptual    = "conceptual"
	dimDecomposition = "decomposition"
	dimCorrectness   = "correctness"
	dimUnderstanding = "understanding"
	dimAIUse         = "ai_use"
	dimStrategy      = "strategy"
)

// modeDimensions lists the rubric dimensions each assessment mode is graded on.
var modeDimensions = map[string][]string{
	modeConceptual: {dimConceptual},
	modeCode:       {dimDecomposition, dimCorrectness, dimUnderstanding, dimAIUse},
	modeBug:        {dimStrategy},
}

type gradeLabel struct {
	Dimension   string
	Level       string
	AnswerIndex int
}

func levelRank(level string) int {
	switch level {
	case levelNotReady:
		return 0
	case levelCompetent:
		return 1
	case levelExceptional:
		return 2
	default:
		return -1
	}
}

// normalizeLevel maps a model-written level to its canonical form, or "" when it is not
// one. Rubric headings say "Not Yet Ready", so that spelling is accepted too.
func normalizeLevel(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.Trim(s, ".*`\"'")
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	switch s {
	case "not_ready", "not_yet_ready", "not_ready_yet", "notready":
		return levelNotReady
	case "competent":
		return levelCompetent
	case "exceptional":
		return levelExceptional
	default:
		return ""
	}
}

func dimensionInMode(mode, dimension string) bool {
	for _, d := range modeDimensions[mode] {
		if d == dimension {
			return true
		}
	}
	return false
}

// lowestLevels returns each dimension's lowest label: when a dimension is labelled more
// than once in a mode, the lowest level wins.
func lowestLevels(labels []gradeLabel) map[string]string {
	out := map[string]string{}
	for _, l := range labels {
		if levelRank(l.Level) < 0 {
			continue
		}
		if prev, ok := out[l.Dimension]; !ok || levelRank(l.Level) < levelRank(prev) {
			out[l.Dimension] = l.Level
		}
	}
	return out
}

func bucketForLevel(level string) string {
	switch level {
	case levelExceptional:
		return bucketExceptional
	case levelCompetent:
		return bucketCompetent
	default:
		return bucketNotReady
	}
}

// gradeMode turns one mode's labels into its bucket (grading-rules.md, Step 2).
// codePasted only matters for Code mode: closing without pasted code makes correctness
// Not Ready.
func gradeMode(mode string, labels []gradeLabel, codePasted bool) string {
	var kept []gradeLabel
	for _, l := range labels {
		if dimensionInMode(mode, l.Dimension) {
			kept = append(kept, l)
		}
	}
	if mode == modeCode && !codePasted {
		kept = append(kept, gradeLabel{Dimension: dimCorrectness, Level: levelNotReady})
	}
	levels := lowestLevels(kept)
	if len(levels) == 0 {
		return bucketNotReady
	}

	switch mode {
	case modeConceptual:
		return bucketForLevel(levels[dimConceptual])
	case modeBug:
		return bucketForLevel(levels[dimStrategy])
	case modeCode:
		if levels[dimCorrectness] == levelNotReady {
			return bucketNotReady
		}
		notReady, allExceptional := 0, true
		for _, level := range levels {
			if level == levelNotReady {
				notReady++
			}
			if level != levelExceptional {
				allExceptional = false
			}
		}
		if notReady >= 2 {
			return bucketNotReady
		}
		if allExceptional {
			return bucketExceptional
		}
		return bucketCompetent
	default:
		return bucketNotReady
	}
}

// applyD5Grades sets the mode buckets and the overall rating on state (Steps 2 and 3).
// The overall rating reuses computeFinalRating, which already follows the final rubric.
func applyD5Grades(state *AgentSessionState, labels map[string][]gradeLabel, codePasted bool) {
	state.ConceptualAssessmentBucket = gradeMode(modeConceptual, labels[modeConceptual], false)
	if state.isProblemDecompositionWeek() {
		state.CodeAssessmentBucket = bucketNA
		state.BugAssessmentBucket = bucketNA
	} else {
		state.CodeAssessmentBucket = gradeMode(modeCode, labels[modeCode], codePasted)
		state.BugAssessmentBucket = gradeMode(modeBug, labels[modeBug], false)
	}
	state.FinalRating = computeFinalRating(state)
	state.AssessmentComplete = state.FinalRating != ""
}
