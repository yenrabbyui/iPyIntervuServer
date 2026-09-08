package main

import (
	"slices"
	"strings"
	"testing"
)

func TestForbiddenConceptsForWeek7(t *testing.T) {
	forbidden := forbiddenConceptsFromLaterWeeks(7)
	if len(forbidden) == 0 {
		t.Fatal("expected forbidden concepts for week 7")
	}
	joined := strings.ToLower(strings.Join(forbidden, " "))
	for _, term := range []string{"lists", "file i/o"} {
		if !strings.Contains(joined, term) {
			t.Fatalf("expected week 7 forbidden list to include %q", term)
		}
	}
}

func TestAllowedWeekNumbersForWeek7(t *testing.T) {
	allowed := allowedWeekNumbers(7)
	want := []int{1, 2, 3, 4, 5, 6, 7}
	if !slices.Equal(allowed, want) {
		t.Fatalf("allowedWeekNumbers(7) = %v, want %v", allowed, want)
	}
}

func TestAssessmentWeekScopeSnapshot(t *testing.T) {
	scope := assessmentWeekScopeSnapshot(7)
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if scope["primaryWeekNumber"] != 7 {
		t.Fatalf("primaryWeekNumber = %v", scope["primaryWeekNumber"])
	}
}
