package main

import "strings"

type weekSelection struct {
	SelectedKeyConcept string
	CurrentWeekNumber  int
}

var weeklyKeyConceptSelections = []weekSelection{
	{SelectedKeyConcept: "Week 1 - Problem Decomposition", CurrentWeekNumber: 1},
	{SelectedKeyConcept: "Week 2 - Variables & Expressions", CurrentWeekNumber: 2},
	{SelectedKeyConcept: "Week 3 - Input & Type Casting", CurrentWeekNumber: 3},
	{SelectedKeyConcept: "Week 4 - String Methods", CurrentWeekNumber: 4},
	{SelectedKeyConcept: "Week 5 - if Statements (Conditionals & Logic)", CurrentWeekNumber: 5},
	{SelectedKeyConcept: "Week 6 - elif/else Statements (Conditionals & Logic)", CurrentWeekNumber: 6},
	{SelectedKeyConcept: "Week 7 - for Loops (Repetition over sequences)", CurrentWeekNumber: 7},
	{SelectedKeyConcept: "Week 8 - while Loops & Menus", CurrentWeekNumber: 8},
	{SelectedKeyConcept: "Week 9 - Lists", CurrentWeekNumber: 9},
	{SelectedKeyConcept: "Week 10 - Lists and Files", CurrentWeekNumber: 10},
}

var weekInputAliases = map[string]weekSelection{
	"1": weekSelection{"Week 1 - Problem Decomposition", 1},
	"week 1": {"Week 1 - Problem Decomposition", 1},
	"problem decomposition": {"Week 1 - Problem Decomposition", 1},
	"2": {"Week 2 - Variables & Expressions", 2},
	"week 2": {"Week 2 - Variables & Expressions", 2},
	"variables": {"Week 2 - Variables & Expressions", 2},
	"variables and expressions": {"Week 2 - Variables & Expressions", 2},
	"3": {"Week 3 - Input & Type Casting", 3},
	"week 3": {"Week 3 - Input & Type Casting", 3},
	"input": {"Week 3 - Input & Type Casting", 3},
	"type casting": {"Week 3 - Input & Type Casting", 3},
	"input and type casting": {"Week 3 - Input & Type Casting", 3},
	"4": {"Week 4 - String Methods", 4},
	"week 4": {"Week 4 - String Methods", 4},
	"strings": {"Week 4 - String Methods", 4},
	"string methods": {"Week 4 - String Methods", 4},
	"5": {"Week 5 - if Statements (Conditionals & Logic)", 5},
	"week 5": {"Week 5 - if Statements (Conditionals & Logic)", 5},
	"if": {"Week 5 - if Statements (Conditionals & Logic)", 5},
	"if statements": {"Week 5 - if Statements (Conditionals & Logic)", 5},
	"conditionals": {"Week 5 - if Statements (Conditionals & Logic)", 5},
	"6": {"Week 6 - elif/else Statements (Conditionals & Logic)", 6},
	"week 6": {"Week 6 - elif/else Statements (Conditionals & Logic)", 6},
	"elif": {"Week 6 - elif/else Statements (Conditionals & Logic)", 6},
	"else": {"Week 6 - elif/else Statements (Conditionals & Logic)", 6},
	"elif else": {"Week 6 - elif/else Statements (Conditionals & Logic)", 6},
	"7": {"Week 7 - for Loops (Repetition over sequences)", 7},
	"week 7": {"Week 7 - for Loops (Repetition over sequences)", 7},
	"for": {"Week 7 - for Loops (Repetition over sequences)", 7},
	"for loops": {"Week 7 - for Loops (Repetition over sequences)", 7},
	"8": {"Week 8 - while Loops & Menus", 8},
	"week 8": {"Week 8 - while Loops & Menus", 8},
	"while": {"Week 8 - while Loops & Menus", 8},
	"while loops": {"Week 8 - while Loops & Menus", 8},
	"menus": {"Week 8 - while Loops & Menus", 8},
	"while loops and menus": {"Week 8 - while Loops & Menus", 8},
	"9": {"Week 9 - Lists", 9},
	"week 9": {"Week 9 - Lists", 9},
	"lists": {"Week 9 - Lists", 9},
	"10": {"Week 10 - Lists and Files", 10},
	"week 10": {"Week 10 - Lists and Files", 10},
	"files": {"Week 10 - Lists and Files", 10},
	"lists and files": {"Week 10 - Lists and Files", 10},
}

func normalizeUserInput(text string) string {
	return strings.TrimSpace(strings.ToLower(text))
}

func matchWeekSelection(userMessage string) (weekSelection, bool) {
	normalized := normalizeUserInput(userMessage)
	if normalized == "" {
		return weekSelection{}, false
	}
	if sel, ok := weekInputAliases[normalized]; ok {
		return sel, true
	}
	for _, concept := range weeklyKeyConceptSelections {
		if strings.EqualFold(strings.TrimSpace(userMessage), concept.SelectedKeyConcept) {
			return concept, true
		}
	}
	return weekSelection{}, false
}

var greetingPhrases = []string{
	"hi", "hello", "hey", "good morning", "good afternoon", "good evening",
	"thanks", "thank you", "ok", "okay", "start",
}

func isGreetingMessage(text string) bool {
	normalized := normalizeUserInput(text)
	for _, phrase := range greetingPhrases {
		if normalized == phrase {
			return true
		}
	}
	return false
}

func isCoachingRequest(text string) bool {
	normalized := normalizeUserInput(text)
	return strings.Contains(normalized, "coaching") ||
		strings.Contains(normalized, "coach mode") ||
		strings.Contains(normalized, "feedback on my assessment") ||
		strings.Contains(normalized, "give me feedback")
}
