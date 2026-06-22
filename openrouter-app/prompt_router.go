package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed env/instructions/IPyIntervu-entrypoint.md
//go:embed env/instructions/IPyIntervu-flow.md
//go:embed env/instructions/IPyIntervu-protocols.md
//go:embed env/instructions/IPyIntervu-week-scope.md
//go:embed env/instructions/IPyIntervu-modes-shared.md
//go:embed env/instructions/IPyIntervu-modes-conceptual.md
//go:embed env/instructions/IPyIntervu-modes-code.md
//go:embed env/instructions/IPyIntervu-modes-bug.md
//go:embed env/instructions/IPyIntervu-modes-coaching.md
//go:embed env/IPYIntervu_support_files/week1_key_concepts.md
//go:embed env/IPYIntervu_support_files/week2_key_concepts.md
//go:embed env/IPYIntervu_support_files/week3_key_concepts.md
//go:embed env/IPYIntervu_support_files/week4_key_concepts.md
//go:embed env/IPYIntervu_support_files/week5_key_concepts.md
//go:embed env/IPYIntervu_support_files/week6_key_concepts.md
//go:embed env/IPYIntervu_support_files/week7_key_concepts.md
//go:embed env/IPYIntervu_support_files/week8_key_concepts.md
//go:embed env/IPYIntervu_support_files/week9_key_concepts.md
//go:embed env/IPYIntervu_support_files/week10_key_concepts.md
//go:embed env/IPYIntervu_support_files/week1_competency_guide.md
//go:embed env/IPYIntervu_support_files/week2_competency_guide.md
//go:embed env/IPYIntervu_support_files/week3_competency_guide.md
//go:embed env/IPYIntervu_support_files/week4_competency_guide.md
//go:embed env/IPYIntervu_support_files/week5_competency_guide.md
//go:embed env/IPYIntervu_support_files/week6_competency_guide.md
//go:embed env/IPYIntervu_support_files/week7_competency_guide.md
//go:embed env/IPYIntervu_support_files/week8_competency_guide.md
//go:embed env/IPYIntervu_support_files/week9_competency_guide.md
//go:embed env/IPYIntervu_support_files/week10_competency_guide.md
//go:embed env/rubrics/week1_rubric.md
//go:embed env/rubrics/week2_rubric.md
//go:embed env/rubrics/week3_rubric.md
//go:embed env/rubrics/week4_rubric.md
//go:embed env/rubrics/week5_rubric.md
//go:embed env/rubrics/week6_rubric.md
//go:embed env/rubrics/week7_rubric.md
//go:embed env/rubrics/week8_rubric.md
//go:embed env/rubrics/week9_rubric.md
//go:embed env/rubrics/week10_rubric.md
//go:embed env/rubrics/final_assessment_rubric.md
var instructionFS embed.FS

type promptFile struct {
	displayName string
	path        string
}

func selectPromptFiles(state *AgentSessionState) ([]promptFile, string) {
	files := []promptFile{
		{displayName: "IPyIntervu-entrypoint.md", path: "env/instructions/IPyIntervu-entrypoint.md"},
	}
	bundleParts := []string{"core"}

	switch state.ConversationPhase {
	case phaseAwaitingMajor, phaseAwaitingKeyConcept:
		files = append(files, promptFile{"IPyIntervu-flow.md", "env/instructions/IPyIntervu-flow.md"})
		bundleParts = append(bundleParts, "flow")
	case phaseAssessmentInProgress, phaseAssessmentResults:
		files = append(files,
			promptFile{"IPyIntervu-protocols.md", "env/instructions/IPyIntervu-protocols.md"},
			promptFile{"IPyIntervu-week-scope.md", "env/instructions/IPyIntervu-week-scope.md"},
			promptFile{"IPyIntervu-modes-shared.md", "env/instructions/IPyIntervu-modes-shared.md"},
		)
		bundleParts = append(bundleParts, "assessment")
	}

	if state.ActiveMode == modeCoaching {
		files = append(files, promptFile{"IPyIntervu-modes-coaching.md", "env/instructions/IPyIntervu-modes-coaching.md"})
		bundleParts = append(bundleParts, "coaching")
	} else if state.ConversationPhase == phaseAssessmentInProgress || state.ConversationPhase == phaseAssessmentResults {
		switch state.ActiveMode {
		case modeConceptual:
			files = append(files, promptFile{"IPyIntervu-modes-conceptual.md", "env/instructions/IPyIntervu-modes-conceptual.md"})
			bundleParts = append(bundleParts, "mode-conceptual")
		case modeCode:
			files = append(files, promptFile{"IPyIntervu-modes-code.md", "env/instructions/IPyIntervu-modes-code.md"})
			bundleParts = append(bundleParts, "mode-code")
		case modeBug:
			files = append(files, promptFile{"IPyIntervu-modes-bug.md", "env/instructions/IPyIntervu-modes-bug.md"})
			bundleParts = append(bundleParts, "mode-bug")
		}
	}

	if state.CurrentWeekNumber > 0 {
		week := state.CurrentWeekNumber
		files = append(files,
			promptFile{fmt.Sprintf("week%d_key_concepts.md", week), fmt.Sprintf("env/IPYIntervu_support_files/week%d_key_concepts.md", week)},
			promptFile{fmt.Sprintf("week%d_competency_guide.md", week), fmt.Sprintf("env/IPYIntervu_support_files/week%d_competency_guide.md", week)},
			promptFile{fmt.Sprintf("week%d_rubric.md", week), fmt.Sprintf("env/rubrics/week%d_rubric.md", week)},
		)
		bundleParts = append(bundleParts, fmt.Sprintf("week%d-kb", week))
	}

	if state.ConversationPhase == phaseAssessmentResults || state.AssessmentComplete {
		files = append(files, promptFile{"final_assessment_rubric.md", "env/rubrics/final_assessment_rubric.md"})
		bundleParts = append(bundleParts, "results")
	}

	return files, strings.Join(bundleParts, "+")
}

func assessmentSyncPromptForState(state *AgentSessionState) string {
	if state.ConversationPhase != phaseAssessmentInProgress {
		return ""
	}
	phaseField, bucketField, label := currentModeSyncFields(state)
	if phaseField == "" {
		return ""
	}
	return fmt.Sprintf(
		"MANDATORY THIS TURN (%s): End your reply with ```_ipyintervu``` as the last lines. "+
			"While interviewing include only {\"%s\": \"in_progress\"}. "+
			"When finishing this mode include \"%s\": \"complete\" and \"%s\". "+
			"Omitting the block forces a server retry and delays the student. Never mention the sync block in interview text.\n",
		label, phaseField, phaseField, bucketField,
	)
}

func buildSystemPrompt(state *AgentSessionState) (string, []string, string, error) {
	files, bundleID := selectPromptFiles(state)
	state.InstructionBundleID = bundleID

	var b strings.Builder
	b.WriteString("IPyIntervu server-managed session state (authoritative; instruction modules are subordinate):\n")
	stateJSON, err := json.MarshalIndent(state.snapshotForPrompt(), "", "  ")
	if err != nil {
		return "", nil, "", err
	}
	b.Write(stateJSON)
	b.WriteString("\n\n")
	b.WriteString("Knowledge-base files injected for this turn appear below. Use only those filenames.\n")
	b.WriteString("Honor assessmentWeekScope in server state: never require concepts from weeks after currentWeekNumber.\n")
	b.WriteString("ASSESSMENT SYNC (mandatory): Every Conceptual/Code/Bug reply MUST end with ```_ipyintervu``` JSON as the last lines — introductions, acknowledgments, and every question. Missing sync triggers a server corrective round-trip.\n")
	b.WriteString("Use assessmentPhase \"in_progress\" while asking interview questions (omit bucket). Use assessmentPhase \"complete\" plus the bucket when finishing that mode. Mode transitions require complete plus a valid bucket.\n")
	if syncLine := assessmentSyncPromptForState(state); syncLine != "" {
		b.WriteString(syncLine)
	}
	b.WriteString("Assessment modes advance forward only (Conceptual → Code → Bug). Never return to a completed or earlier mode; follow activeMode in server state.\n")
	b.WriteString("During assessment (coachingRequested false): never offer explanations, walkthroughs, hints that reveal answers, or coaching. Only Coaching mode may explain or teach.\n")
	if state.ConversationPhase == phaseAssessmentInProgress {
		b.WriteString("Never answer your own questions: ask one question, append the silent _ipyintervu block, then STOP. Do not supply the answer, model response, solution code, or the bug/fix, and never write or simulate the student's reply. Wait for an actual user message before continuing.\n")
	}
	if state.ActiveMode == modeBug && state.ConversationPhase == phaseAssessmentInProgress {
		b.WriteString("Bug Hunting: ask how the student would find the bug (process only). Do not ask for corrected/fixed code. Do not answer your own debugging questions.\n")
	}
	b.WriteString("\n")

	loaded := make([]string, 0, len(files))
	for _, file := range files {
		data, err := instructionFS.ReadFile(file.path)
		if err != nil {
			return "", nil, "", fmt.Errorf("read %s: %w", file.path, err)
		}
		b.WriteString("=== ")
		b.WriteString(file.displayName)
		b.WriteString(" ===\n")
		b.Write(data)
		b.WriteString("\n\n")
		loaded = append(loaded, file.displayName)
	}

	state.KBFilesLoaded = loaded
	return b.String(), loaded, bundleID, nil
}
