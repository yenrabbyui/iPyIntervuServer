package main

import (
	"slices"
	"strings"
	"testing"
)

func TestForbiddenConceptsForWeek8(t *testing.T) {
	forbidden := forbiddenConceptsFromLaterWeeks(8)
	if len(forbidden) == 0 {
		t.Fatal("expected forbidden concepts for week 8")
	}
	joined := strings.ToLower(strings.Join(forbidden, " "))
	for _, term := range []string{"lists", "file i/o", "dictionaries"} {
		if !strings.Contains(joined, term) {
			t.Fatalf("expected week 8 forbidden list to include %q", term)
		}
	}
}

func TestAllowedWeekNumbersForWeek8(t *testing.T) {
	allowed := allowedWeekNumbers(8)
	want := []int{1, 2, 3, 4, 5, 6, 7, 8}
	if !slices.Equal(allowed, want) {
		t.Fatalf("allowedWeekNumbers(8) = %v, want %v", allowed, want)
	}
}

func TestAssessmentWeekScopeSnapshot(t *testing.T) {
	scope := assessmentWeekScopeSnapshot(8)
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if scope["primaryWeekNumber"] != 8 {
		t.Fatalf("primaryWeekNumber = %v", scope["primaryWeekNumber"])
	}
}
