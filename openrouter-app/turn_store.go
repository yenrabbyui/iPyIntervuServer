package main

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	turnIDHeader     = "X-Turn-Id"
	bootstrapTurnID  = "ipyintervu-bootstrap"
	maxTurnsPerSession = 32
)

type turnStatus string

const (
	turnInProgress turnStatus = "in_progress"
	turnCompleted  turnStatus = "completed"
	turnFailed     turnStatus = "failed"
)

type turnRole int

const (
	turnRoleLeader turnRole = iota
	turnRoleFollower
	turnRoleReplayCompleted
	turnRoleReplayFailed
	turnRoleResumeLeader
)

type turnAcquireResult struct {
	record *chatTurnRecord
	role   turnRole
}

type sharedTurnStream struct {
	mu               sync.Mutex
	chunks           []string
	visibleAssistant string
	rawAssistant     string
	done             bool
	failed           bool
	notify           chan struct{}
	resumeClaimed    bool
}

func newSharedTurnStream() *sharedTurnStream {
	return &sharedTurnStream{
		notify: make(chan struct{}, 1),
	}
}

func (s *sharedTurnStream) signalWaiters() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *sharedTurnStream) appendChunk(ssePart, rawAssistant, visibleAssistant string) {
	s.mu.Lock()
	s.chunks = append(s.chunks, ssePart)
	s.rawAssistant = rawAssistant
	s.visibleAssistant = visibleAssistant
	s.mu.Unlock()
	s.signalWaiters()
}

func (s *sharedTurnStream) markDone(rawAssistant, visibleAssistant string, failed bool) {
	s.mu.Lock()
	s.rawAssistant = rawAssistant
	s.visibleAssistant = visibleAssistant
	s.done = true
	s.failed = failed
	s.mu.Unlock()
	s.signalWaiters()
}

func (s *sharedTurnStream) follow(w http.ResponseWriter, ctx context.Context) error {
	idx := 0
	keepalive := time.NewTicker(12 * time.Second)
	defer keepalive.Stop()
	for {
		s.mu.Lock()
		for idx < len(s.chunks) {
			part := s.chunks[idx]
			idx++
			s.mu.Unlock()
			if _, err := w.Write([]byte(part + "\n\n")); err != nil {
				return err
			}
			flushResponseWriter(w)
			s.mu.Lock()
		}
		done := s.done
		s.mu.Unlock()
		if done {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.notify:
		case <-keepalive.C:
			writeSSEKeepalive(w)
		}
	}
}

func (s *sharedTurnStream) replayExistingTo(w io.Writer) error {
	s.mu.Lock()
	chunks := append([]string(nil), s.chunks...)
	s.mu.Unlock()
	for _, part := range chunks {
		if _, err := w.Write([]byte(part + "\n\n")); err != nil {
			return err
		}
		flushResponseWriter(w)
	}
	return nil
}

func (s *sharedTurnStream) resetResumeClaim() {
	s.mu.Lock()
	s.resumeClaimed = false
	s.mu.Unlock()
}

func (s *sharedTurnStream) tryClaimResume() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resumeClaimed || !s.failed || !s.done {
		return false
	}
	s.resumeClaimed = true
	s.done = false
	s.failed = false
	return true
}

type chatTurnRecord struct {
	turnID    string
	status    turnStatus
	stream    *sharedTurnStream
	createdAt time.Time
	updatedAt time.Time
}

type bootstrapFlight struct {
	inFlight bool
	done     bool
	notify   chan struct{}
}

type turnStore struct {
	mu              sync.Mutex
	sessions        map[string]map[string]*chatTurnRecord
	bootstrap       map[string]string // sessionID -> assistant text
	bootstrapFlight map[string]*bootstrapFlight
}

func newTurnStore() *turnStore {
	store := &turnStore{
		sessions:        make(map[string]map[string]*chatTurnRecord),
		bootstrap:       make(map[string]string),
		bootstrapFlight: make(map[string]*bootstrapFlight),
	}
	go store.cleanupLoop()
	return store
}

func (s *turnStore) sessionTurns(sessionID string) map[string]*chatTurnRecord {
	if s.sessions[sessionID] == nil {
		s.sessions[sessionID] = make(map[string]*chatTurnRecord)
	}
	return s.sessions[sessionID]
}

func (s *turnStore) acquire(sessionID, turnID string) turnAcquireResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	turns := s.sessionTurns(sessionID)
	rec, ok := turns[turnID]
	if !ok {
		rec = &chatTurnRecord{
			turnID:    turnID,
			status:    turnInProgress,
			stream:    newSharedTurnStream(),
			createdAt: time.Now(),
			updatedAt: time.Now(),
		}
		turns[turnID] = rec
		s.trimTurns(sessionID)
		return turnAcquireResult{record: rec, role: turnRoleLeader}
	}

	switch rec.status {
	case turnInProgress:
		return turnAcquireResult{record: rec, role: turnRoleFollower}
	case turnCompleted:
		return turnAcquireResult{record: rec, role: turnRoleReplayCompleted}
	case turnFailed:
		if rec.stream.tryClaimResume() {
			rec.status = turnInProgress
			rec.updatedAt = time.Now()
			return turnAcquireResult{record: rec, role: turnRoleResumeLeader}
		}
		return turnAcquireResult{record: rec, role: turnRoleReplayFailed}
	default:
		return turnAcquireResult{record: rec, role: turnRoleLeader}
	}
}

func (s *turnStore) trimTurns(sessionID string) {
	turns := s.sessions[sessionID]
	if len(turns) <= maxTurnsPerSession {
		return
	}
	var oldestID string
	var oldest time.Time
	for id, rec := range turns {
		if oldestID == "" || rec.createdAt.Before(oldest) {
			oldestID = id
			oldest = rec.createdAt
		}
	}
	delete(turns, oldestID)
}

func (s *turnStore) completeTurn(sessionID, turnID, rawAssistant, visibleAssistant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.sessions[sessionID][turnID]
	if rec == nil {
		return
	}
	rec.status = turnCompleted
	rec.updatedAt = time.Now()
	rec.stream.markDone(rawAssistant, visibleAssistant, false)
}

func (s *turnStore) failTurn(sessionID, turnID, rawAssistant, visibleAssistant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.sessions[sessionID][turnID]
	if rec == nil {
		return
	}
	rec.status = turnFailed
	rec.updatedAt = time.Now()
	rec.stream.resetResumeClaim()
	rec.stream.markDone(rawAssistant, visibleAssistant, true)
}

func (s *turnStore) getBootstrapAssistant(sessionID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assistant, ok := s.bootstrap[sessionID]
	return assistant, ok
}

func (s *turnStore) setBootstrapAssistant(sessionID, assistant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrap[sessionID] = assistant
	if flight := s.bootstrapFlight[sessionID]; flight != nil {
		flight.inFlight = false
		flight.done = true
		closeBootstrapNotify(flight)
	}
}

func closeBootstrapNotify(flight *bootstrapFlight) {
	select {
	case flight.notify <- struct{}{}:
	default:
	}
}

func (s *turnStore) beginBootstrap(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bootstrap[sessionID]; ok {
		return false
	}
	flight := s.bootstrapFlight[sessionID]
	if flight == nil {
		flight = &bootstrapFlight{notify: make(chan struct{}, 1)}
		s.bootstrapFlight[sessionID] = flight
	}
	if flight.inFlight {
		return false
	}
	flight.inFlight = true
	return true
}

func (s *turnStore) waitBootstrap(ctx context.Context, sessionID string) (string, error) {
	for {
		s.mu.Lock()
		if assistant, ok := s.bootstrap[sessionID]; ok {
			s.mu.Unlock()
			return assistant, nil
		}
		flight := s.bootstrapFlight[sessionID]
		notify := flight.notify
		s.mu.Unlock()
		if notify == nil {
			return "", context.Canceled
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-notify:
		}
	}
}

func (s *turnStore) cancelBootstrap(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if flight := s.bootstrapFlight[sessionID]; flight != nil {
		flight.inFlight = false
		closeBootstrapNotify(flight)
	}
}

func (s *turnStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-25 * time.Hour)
		s.mu.Lock()
		for sessionID, turns := range s.sessions {
			for turnID, rec := range turns {
				if rec.updatedAt.Before(cutoff) {
					delete(turns, turnID)
				}
			}
			if len(turns) == 0 {
				delete(s.sessions, sessionID)
			}
		}
		for sessionID := range s.bootstrap {
			// bootstrap entries expire with session turns cleanup pass
			if _, ok := s.sessions[sessionID]; !ok {
				delete(s.bootstrap, sessionID)
			}
		}
		s.mu.Unlock()
	}
}

func flushResponseWriter(w io.Writer) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
