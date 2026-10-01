package main

import "testing"

func labelsOf(dim string, levels ...string) []gradeLabel {
	out := make([]gradeLabel, len(levels))
	for i, l := range levels {
		out[i] = gradeLabel{Dimension: dim, Level: l, AnswerIndex: i + 1}
	}
	return out
}

func codeLabels(pairs ...string) []gradeLabel {
	var out []gradeLabel
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, gradeLabel{Dimension: pairs[i], Level: pairs[i+1]})
	}
	return out
}

// The cases are the worked examples in grading-rules.md, numbered the same way.
func TestGradeModeSpecExamples(t *testing.T) {
	const nr, c, e = levelNotReady, levelCompetent, levelExceptional
	cases := []struct {
		n          int
		mode       string
		labels     []gradeLabel
		codePasted bool
		want       string
	}{
		{1, modeConceptual, labelsOf(dimConceptual, c, e, e), false, bucketCompetent},
		{2, modeConceptual, labelsOf(dimConceptual, nr, nr, c), false, bucketNotReady},
		{3, modeConceptual, labelsOf(dimConceptual, e, e, e), false, bucketExceptional},
		{4, modeConceptual, labelsOf(dimConceptual, nr, e, e), false, bucketNotReady},
		{5, modeConceptual, nil, false, bucketNotReady},
		{6, modeBug, labelsOf(dimStrategy, c, e, c), false, bucketCompetent},
		{7, modeBug, labelsOf(dimStrategy, e, e, e, e), false, bucketExceptional},
		{8, modeCode, codeLabels(dimDecomposition, e, dimCorrectness, e, dimUnderstanding, e), true, bucketExceptional},
		{9, modeCode, codeLabels(dimDecomposition, e, dimCorrectness, c, dimUnderstanding, e, dimAIUse, e), true, bucketCompetent},
		{10, modeCode, codeLabels(dimCorrectness, nr, dimDecomposition, e, dimUnderstanding, e, dimAIUse, e), true, bucketNotReady},
		{11, modeCode, codeLabels(dimDecomposition, nr, dimUnderstanding, nr, dimCorrectness, c), true, bucketNotReady},
		{12, modeCode, codeLabels(dimAIUse, nr, dimDecomposition, e, dimCorrectness, e, dimUnderstanding, e), true, bucketCompetent},
		{13, modeCode, codeLabels(dimUnderstanding, e, dimUnderstanding, nr, dimDecomposition, c, dimCorrectness, c, dimAIUse, c), true, bucketCompetent},
		{14, modeCode, codeLabels(dimDecomposition, e), false, bucketNotReady},
	}
	for _, tc := range cases {
		if got := gradeMode(tc.mode, tc.labels, tc.codePasted); got != tc.want {
			t.Errorf("example %d: gradeMode = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestGradeModeIgnoresOtherModesDimensions(t *testing.T) {
	labels := []gradeLabel{{Dimension: dimConceptual, Level: levelExceptional}, {Dimension: dimAIUse, Level: levelNotReady}}
	if got := gradeMode(modeConceptual, labels, false); got != bucketExceptional {
		t.Fatalf("got %q, want Exceptional", got)
	}
}

func TestApplyD5GradesOverallRating(t *testing.T) {
	const nr, c, e = levelNotReady, levelCompetent, levelExceptional
	codeAll := func(level string) []gradeLabel {
		return codeLabels(dimDecomposition, level, dimCorrectness, level, dimUnderstanding, level, dimAIUse, level)
	}
	cases := []struct {
		n      int
		week   int
		labels map[string][]gradeLabel
		want   [4]string
	}{
		{15, 5, map[string][]gradeLabel{
			modeConceptual: labelsOf(dimConceptual, c),
			modeCode:       codeAll(e),
			modeBug:        labelsOf(dimStrategy, e),
		}, [4]string{bucketCompetent, bucketExceptional, bucketExceptional, bucketCompetent}},
		{16, 5, map[string][]gradeLabel{
			modeConceptual: labelsOf(dimConceptual, e),
			modeCode:       codeAll(e),
			modeBug:        labelsOf(dimStrategy, nr),
		}, [4]string{bucketExceptional, bucketExceptional, bucketNotReady, bucketNotReady}},
		{17, 5, map[string][]gradeLabel{
			modeConceptual: labelsOf(dimConceptual, e),
			modeCode:       codeAll(e),
			modeBug:        labelsOf(dimStrategy, e),
		}, [4]string{bucketExceptional, bucketExceptional, bucketExceptional, bucketExceptional}},
		{18, 1, map[string][]gradeLabel{
			modeConceptual: labelsOf(dimConceptual, c),
		}, [4]string{bucketCompetent, bucketNA, bucketNA, bucketCompetent}},
	}
	for _, tc := range cases {
		state := newAgentSessionState()
		state.CurrentWeekNumber = tc.week
		applyD5Grades(state, tc.labels, true)
		got := [4]string{state.ConceptualAssessmentBucket, state.CodeAssessmentBucket, state.BugAssessmentBucket, state.FinalRating}
		if got != tc.want {
			t.Errorf("example %d: got %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestNormalizeLevel(t *testing.T) {
	for raw, want := range map[string]string{
		"not_ready": levelNotReady, "Not Yet Ready": levelNotReady, "not ready yet": levelNotReady,
		"Competent": levelCompetent, "**exceptional**": levelExceptional, "strong": "",
	} {
		if got := normalizeLevel(raw); got != want {
			t.Errorf("normalizeLevel(%q) = %q, want %q", raw, got, want)
		}
	}
}
