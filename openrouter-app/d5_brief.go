package main

import (
	"regexp"
	"strings"
)

// d5Brief is the Evaluator's labelled-line output (design §5), parsed tolerantly.
type d5Brief struct {
	Mode          string
	AnswerIndex   int
	Quality       string
	Clarification bool
	Evidence      []string
	Covered       []string
	Gaps          []string
	Next          string
	Fallback      string
	Avoid         []string
	Recommend     string // "continue" or "close"
	Levels        []gradeLabel
	Issues        []string
}

var labelledLinePrefix = regexp.MustCompile(`^[\s>*_-]*`)

// splitLabelledLine splits "LABEL: value" (allowing markdown bold, bullets and "- "
// prefixes) into an upper-case label and its value.
func splitLabelledLine(line string) (string, string, bool) {
	line = labelledLinePrefix.ReplaceAllString(line, "")
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	label := strings.ToUpper(strings.Trim(strings.TrimSpace(line[:idx]), "*_`"))
	if strings.ContainsAny(label, " \t") && label != "AI USE" {
		return "", "", false
	}
	value := strings.TrimSpace(strings.Trim(strings.TrimSpace(line[idx+1:]), "*_`"))
	return label, value, true
}

func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ";") {
		item = strings.TrimSpace(item)
		if item == "" || strings.EqualFold(item, "none") || strings.EqualFold(item, "n/a") {
			continue
		}
		out = append(out, item)
	}
	return out
}

// firstWord returns the first word of an enumerated value, so "close (enough evidence)"
// reads as "close".
func firstWord(value string) string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ' ' || r == '(' || r == ',' || r == '.' || r == '|'
	})
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// parseLevels reads "dim=level; dim=level" for the given mode, keeping only the mode's
// dimensions and valid levels.
func parseLevels(value, mode string, answerIndex int) []gradeLabel {
	var out []gradeLabel
	for _, pair := range strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == ',' }) {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			k, v, ok = strings.Cut(pair, ":")
		}
		if !ok {
			continue
		}
		dim := strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(k), "*`")))
		dim = strings.NewReplacer(" ", "_", "-", "_").Replace(dim)
		level := normalizeLevel(v)
		if level == "" || !dimensionInMode(mode, dim) {
			continue
		}
		out = append(out, gradeLabel{Dimension: dim, Level: level, AnswerIndex: answerIndex})
	}
	return out
}

// parseBrief parses an Evaluator brief. ok is false when the brief has neither NEXT nor
// FALLBACK, in which case the caller keeps the previous brief.
func parseBrief(text, mode string, answerIndex int) (d5Brief, bool) {
	b := d5Brief{Mode: mode, AnswerIndex: answerIndex, Recommend: "continue"}
	for _, line := range strings.Split(text, "\n") {
		label, value, ok := splitLabelledLine(line)
		if !ok {
			continue
		}
		switch label {
		case "QUALITY":
			switch q := firstWord(value); q {
			case "vague", "partial", "solid", "strong":
				b.Quality = q
			}
		case "CLARIFICATION":
			b.Clarification = firstWord(value) == "yes"
		case "EVIDENCE":
			b.Evidence = splitList(value)
		case "COVERED":
			b.Covered = splitList(value)
		case "GAPS":
			b.Gaps = splitList(value)
		case "NEXT":
			b.Next = value
		case "FALLBACK":
			b.Fallback = value
		case "AVOID":
			b.Avoid = splitList(value)
		case "RECOMMEND":
			if firstWord(value) == "close" {
				b.Recommend = "close"
			}
		case "LEVELS":
			b.Levels = parseLevels(value, mode, answerIndex)
		case "ISSUES":
			b.Issues = splitList(value)
		}
	}
	return b, b.Next != "" || b.Fallback != ""
}

// parseLevelsLine finds the LEVELS line in a levels-only reply.
func parseLevelsLine(text, mode string, answerIndex int) []gradeLabel {
	for _, line := range strings.Split(text, "\n") {
		if label, value, ok := splitLabelledLine(line); ok && label == "LEVELS" {
			return parseLevels(value, mode, answerIndex)
		}
	}
	return nil
}

// parseCompanyLine splits the first opening's "COMPANY: name | description" line from
// the rest of the reply. ok is false when the line is missing.
func parseCompanyLine(reply string) (name, domain, rest string, ok bool) {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		label, value, found := splitLabelledLine(line)
		if !found || label != "COMPANY" {
			return "", "", strings.TrimSpace(reply), false
		}
		name, domain, _ = strings.Cut(value, "|")
		if !strings.Contains(value, "|") {
			name, domain, _ = strings.Cut(value, " - ")
		}
		name = strings.TrimSpace(strings.Trim(name, "*`\""))
		domain = strings.TrimSpace(strings.TrimRight(strings.Trim(domain, "*`\""), "."))
		rest = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		return name, domain, rest, name != ""
	}
	return "", "", "", false
}
