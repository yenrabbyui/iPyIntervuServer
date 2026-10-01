package main

import (
	"strings"
	"testing"
)

func TestD5AskedForClarification(t *testing.T) {
	for msg, want := range map[string]bool{
		"what do you mean by risk band?":                                true,
		"Sorry, can you rephrase that?":                                 true,
		"Do you want the steps or the code?":                            true,
		"Would I check the input first? I'd add a print before the if.": false,
		"I'd compare the score to each threshold.":                      false,
		"Would I check the input first?":                                true,
		"I think it's the order of the elif checks?":                    false,
		strings.Repeat("word ", 30) + "?":                               false,
	} {
		if got := d5AskedForClarification(msg); got != want {
			t.Errorf("d5AskedForClarification(%q) = %v, want %v", msg, got, want)
		}
	}
}

func TestD5ScopeFixes(t *testing.T) {
	if found := d5OutOfScopeConcepts(1, "The user types in each client's allergies, and the planner turns them into meals."); len(found) != 0 {
		t.Fatalf("week 1 'user types' flagged: %v", found)
	}
	if found := d5OutOfScopeConcepts(2, "The user types in each client's weight."); len(found) == 0 {
		t.Fatal("week 2 'user types' should still be flagged as input()")
	}
	if found := d5OutOfScopeConcepts(5, "We keep customer files for every patient visit."); len(found) != 0 {
		t.Fatalf("business wording 'customer files' flagged: %v", found)
	}
	if found := d5OutOfScopeConcepts(5, "The script should read the data file each morning."); len(found) == 0 {
		t.Fatal("reading a data file should be flagged before week 9")
	}
	if found := d5OutOfScopeConcepts(9, "Store the totals in a dictionary."); len(found) == 0 {
		t.Fatal("dictionaries are never in scope")
	}
}

func TestPromptsBannedLoopControl(t *testing.T) {
	if !promptsBannedLoopControl(7, "Could you rewrite it with a `while True` loop and a break?", "") {
		t.Fatal("week 7 prompt for while True not flagged")
	}
	if promptsBannedLoopControl(7, "Why did you use `break` there?", "while True:\n    if x == 'q':\n        break") {
		t.Fatal("asking about the candidate's own break should be allowed")
	}
	if promptsBannedLoopControl(6, "Could you use break?", "") {
		t.Fatal("the ban starts in week 7")
	}
	if promptsBannedLoopControl(8, "Let's continue with the next question about your list.", "") {
		t.Fatal("prose 'continue' is not the continue statement")
	}
}

func TestCorrectnessVerdictAllowsWarmth(t *testing.T) {
	if correctnessVerdictPattern.MatchString("Thanks, that's a helpful way to think about it. What would you check next?") {
		t.Fatal("warm phrasing flagged")
	}
	for _, s := range []string{"That's exactly right. What next?", "You're correct about the order.", "Exactly! Now, what about the fee?", "Spot on."} {
		if !correctnessVerdictPattern.MatchString(s) {
			t.Errorf("verdict not flagged: %q", s)
		}
	}
}

func TestD5SimulatedReplyCut(t *testing.T) {
	text := "Thanks for that. How would you test it?\nCandidate: I would try 50 dollars."
	idx := d5SimulatedReplyCut(text)
	if idx < 0 || strings.TrimSpace(text[:idx]) != "Thanks for that. How would you test it?" {
		t.Fatalf("cut at %d", idx)
	}
	if d5SimulatedReplyCut("Here's what I'd like from you: a short plan. What comes first?") >= 0 {
		t.Fatal("prose 'from you:' must not be cut")
	}
}

func TestD5PostCheckAppendsFallbackWhenNoQuestion(t *testing.T) {
	state, sess := d5TestState(5, modeConceptual)
	res := d5PostCheck(state, sess, modeConceptual, d5Move{Kind: moveFollowUp}, "Thanks, that's a thoughtful approach.", "What would happen on the boundary?")
	if !strings.HasSuffix(res.Reply, "What would happen on the boundary?") {
		t.Fatalf("reply = %q", res.Reply)
	}
	if len(res.Issues) == 0 {
		t.Fatal("missing question should be reported")
	}
}

func TestD5PostCheckFlagsInstructorAndPersona(t *testing.T) {
	state, sess := d5TestState(5, modeConceptual)
	sess.Personas[modeConceptual] = d5Persona{Name: "Jordan", Role: "hiring manager"}
	res := d5PostCheck(state, sess, modeConceptual, d5Move{Kind: moveFollowUp}, "As your instructor, I'd like to hear more. Taylor, would you like to begin?", "")
	joined := strings.Join(res.Issues, "|")
	if !strings.Contains(joined, "instructor") || !strings.Contains(joined, "another interviewer") {
		t.Fatalf("issues = %v", res.Issues)
	}
}

func TestD5LooksLikeCodeSubmission(t *testing.T) {
	for msg, want := range map[string]bool{
		"First read the subtotal, then apply the discount if it is over 50, then print the total.": false,
		"I'd loop for each item and add it up.":                                                    false,
		"```python\nprint('hi')\n```":                                                              true,
		"subtotal = float(input('Subtotal: '))\nprint(subtotal * 0.9)":                             true,
		"total = price * qty":                                                                      false,
		"total = float(input('Price: '))":                                                          true,
		"for item in range(count):\n    total += price":                                            true,
	} {
		if got := d5LooksLikeCodeSubmission(msg); got != want {
			t.Errorf("d5LooksLikeCodeSubmission(%q) = %v, want %v", msg, got, want)
		}
	}
}
