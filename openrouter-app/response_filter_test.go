package main

import "testing"

func TestStripIPyIntervuTail(t *testing.T) {
	raw := "Hi, I'm Alex at ChemSolve Labs.\n\n```_ipyintervu\n{\n  \"businessDomain\": {\"companyName\": \"ChemSolve Labs\", \"domain\": \"chemistry lab simulation software\"}\n}\n```"
	got := stripIPyIntervuTail(raw)
	want := "Hi, I'm Alex at ChemSolve Labs."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestClientVisibleAssistantContentHoldsPartialFence(t *testing.T) {
	raw := "Hello\n\n```_ipyintervu\n{\"businessDomain\":"
	got := clientVisibleAssistantContent(raw)
	want := "Hello"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestClientVisibleAssistantContentHoldsPartialIPyFence(t *testing.T) {
	raw := "Understood.\n\nI think we have a solid picture.\n\n```_ipy"
	got := clientVisibleAssistantContent(raw)
	want := "Understood.\n\nI think we have a solid picture."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripIPyIntervuTailShortFenceTag(t *testing.T) {
	raw := "Summary text.\n\n```_ipy\n{\"conceptualAssessmentBucket\": \"Competent\"}\n```"
	got := stripIPyIntervuTail(raw)
	want := "Summary text."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
