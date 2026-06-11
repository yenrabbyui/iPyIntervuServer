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

	if state.CoachingRequested {
		files = append(files, promptFile{"IPyIntervu-modes-coaching.md", "env/instructions/IPyIntervu-modes-coaching.md"})
		bundleParts = append(bundleParts, "coaching")
	} else {
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
	b.WriteString("When you assign assessment buckets or businessDomain, include a fenced ```_ipyintervu``` JSON block at the end of your reply for server state sync.\n\n")

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
