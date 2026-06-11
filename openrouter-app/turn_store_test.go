package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTurnStoreAcquireLeaderAndFollower(t *testing.T) {
	store := newTurnStore()
	sessionID := "sess-1"
	turnID := "turn-a"

	first := store.acquire(sessionID, turnID)
	if first.role != turnRoleLeader {
		t.Fatalf("first role = %v, want leader", first.role)
	}
	if first.record.status != turnInProgress {
		t.Fatalf("status = %q, want in_progress", first.record.status)
	}

	second := store.acquire(sessionID, turnID)
	if second.role != turnRoleFollower {
		t.Fatalf("second role = %v, want follower", second.role)
	}

	store.completeTurn(sessionID, turnID, "raw assistant", "visible assistant")

	third := store.acquire(sessionID, turnID)
	if third.role != turnRoleReplayCompleted {
		t.Fatalf("third role = %v, want replay_completed", third.role)
	}
}

func TestSharedTurnStreamFollow(t *testing.T) {
	stream := newSharedTurnStream()
	stream.appendChunk("data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}", "Hi", "Hi")
	stream.markDone("Hi there", "Hi there", false)

	rec := httptest.NewRecorder()
	if err := stream.follow(rec, context.Background()); err != nil {
		t.Fatalf("follow returned error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Hi") {
		t.Fatalf("unexpected follow body: %q", body)
	}
}

func TestSharedTurnStreamFollowLive(t *testing.T) {
	stream := newSharedTurnStream()
	done := make(chan error, 1)
	rec := httptest.NewRecorder()

	go func() {
		done <- stream.follow(rec, context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	stream.appendChunk("data: {\"choices\":[{\"delta\":{\"content\":\"Live\"}}]}", "Live", "Live")
	stream.markDone("Live", "Live", false)

	if err := <-done; err != nil {
		t.Fatalf("follow returned error: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "Live") {
		t.Fatalf("missing live chunk: %q", rec.Body.String())
	}
}

func TestTurnStoreFailedPartialResume(t *testing.T) {
	store := newTurnStore()
	sessionID := "sess-2"
	turnID := "turn-b"

	store.acquire(sessionID, turnID)
	store.failTurn(sessionID, turnID, "partial raw", "partial visible")

	resume := store.acquire(sessionID, turnID)
	if resume.role != turnRoleResumeLeader {
		t.Fatalf("resume role = %v, want resume_leader", resume.role)
	}

	follower := store.acquire(sessionID, turnID)
	if follower.role != turnRoleFollower {
		t.Fatalf("follower role = %v, want follower", follower.role)
	}
}

func TestBootstrapFlight(t *testing.T) {
	store := newTurnStore()
	sessionID := "sess-3"

	if !store.beginBootstrap(sessionID) {
		t.Fatal("expected bootstrap leader")
	}
	if store.beginBootstrap(sessionID) {
		t.Fatal("expected bootstrap follower while in flight")
	}

	store.setBootstrapAssistant(sessionID, "hello")
	if assistant, ok := store.getBootstrapAssistant(sessionID); !ok || assistant != "hello" {
		t.Fatalf("cached bootstrap = %q, ok=%v", assistant, ok)
	}
}

func TestWriteSSEReplayFromVisible(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := writeSSEReplayFromVisible(rec, "Hello world"); err != nil {
		t.Fatalf("writeSSEReplayFromVisible: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Hello world") {
		t.Fatalf("missing content in replay: %q", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("missing DONE in replay: %q", body)
	}
}
