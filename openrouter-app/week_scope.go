package main

// lastSyllabusWeek is the final week of the CSE 110 syllabus.
const lastSyllabusWeek = 9

// CSE 110 syllabus: concepts first introduced each week (weeks 1..N are allowed support).
var conceptsIntroducedByWeek = map[int][]string{
	1: {
		"problem decomposition",
		"input / process / output",
		"breaking tasks into steps",
	},
	2: {
		"variables",
		"expressions",
		"assignment",
		"int, float, str, bool",
	},
	3: {
		"input()",
		"type casting",
		"int()",
		"float()",
		"str()",
	},
	4: {
		"string methods",
		"strip()",
		"upper()",
		"lower()",
		"split()",
		"replace()",
	},
	5: {
		"if statements",
		"elif",
		"else",
		"multi-branch conditionals",
		"comparison operators",
		"boolean expressions",
		"and",
		"or",
		"not",
	},
	6: {
		"for loops",
		"range()",
		"iterating over sequences",
	},
	7: {
		"while loops (condition in header)",
		"menus",
		"repeat until quit via while condition",
	},
	8: {
		"lists",
		"list indexing",
		"list methods (.append, .remove, .sort, etc.)",
		"len() on lists",
	},
	9: {
		"file I/O",
		"open()",
		"read()",
		"write()",
		"with open(...) for files",
		"reading and writing data files",
	},
}

func allowedWeekNumbers(currentWeek int) []int {
	if currentWeek < 1 {
		return nil
	}
	weeks := make([]int, currentWeek)
	for i := 1; i <= currentWeek; i++ {
		weeks[i-1] = i
	}
	return weeks
}

func forbiddenConceptsFromLaterWeeks(currentWeek int) []string {
	if currentWeek < 1 {
		return nil
	}
	var forbidden []string
	for week := currentWeek + 1; week <= lastSyllabusWeek; week++ {
		forbidden = append(forbidden, conceptsIntroducedByWeek[week]...)
	}
	return forbidden
}

func assessmentWeekScopeSnapshot(currentWeek int) map[string]any {
	if currentWeek < 1 {
		return nil
	}
	return map[string]any{
		"allowedWeekNumbers":              allowedWeekNumbers(currentWeek),
		"primaryWeekNumber":               currentWeek,
		"forbiddenConceptsFromLaterWeeks": forbiddenConceptsFromLaterWeeks(currentWeek),
		"scopeRule":                       "All conceptual questions, code tasks, bug snippets, and follow-ups must use only concepts from allowedWeekNumbers (weeks 1 through primaryWeekNumber). Never require, assume, or prompt for forbiddenConceptsFromLaterWeeks. Prior-week concepts may appear as supporting ingredients; later-week concepts must not appear even as optional extras. Revise any draft that violates this before presenting it.",
	}
}
