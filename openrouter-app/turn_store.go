package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const (
	turnIDHeader       = "X-Turn-Id"
	bootstrapTurnID    = "ipyintervu-bootstrap"
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

type turnWaitState struct {
	mu               sync.Mutex
	done             bool
	failed           bool
	rawAssistant     string
	visibleAssistant string
	responseBody     []byte
	statusCode       int
	notify           chan struct{}
	resumeClaimed    bool
}

func newTurnWaitState() *turnWaitState {
	return &turnWaitState{
		notify: make(chan struct{}, 1),
	}
}

func (s *turnWaitState) signalWaiters() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *turnWaitState) markDone(rawAssistant, visibleAssistant string, responseBody []byte, statusCode int, failed bool) {
	s.mu.Lock()
	s.rawAssistant = rawAssistant
	s.visibleAssistant = visibleAssistant
	s.responseBody = responseBody
	s.statusCode = statusCode
	s.done = true
	s.failed = failed
	s.mu.Unlock()
	s.signalWaiters()
}

func (s *turnWaitState) wait(ctx context.Context) bool {
	for {
		s.mu.Lock()
		if s.done {
			s.mu.Unlock()
			return true
		}
		notify := s.notify
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return false
		case <-notify:
		}
	}
}

func (s *turnWaitState) snapshot() (responseBody []byte, statusCode int, failed bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		return nil, 0, false, false
	}
	return s.responseBody, s.statusCode, s.failed, true
}

func (s *turnWaitState) resetResumeClaim() {
	s.mu.Lock()
	s.resumeClaimed = false
	s.mu.Unlock()
}

func (s *turnWaitState) tryClaimResume() bool {
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
	wait      *turnWaitState
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
			wait:      newTurnWaitState(),
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
		if rec.wait.tryClaimResume() {
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

func (s *turnStore) completeTurn(sessionID, turnID, rawAssistant, visibleAssistant string, responseBody []byte, statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.sessions[sessionID][turnID]
	if rec == nil {
		return
	}
	rec.status = turnCompleted
	rec.updatedAt = time.Now()
	rec.wait.markDone(rawAssistant, visibleAssistant, responseBody, statusCode, false)
}

func (s *turnStore) failTurn(sessionID, turnID, rawAssistant, visibleAssistant string, responseBody []byte, statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.sessions[sessionID][turnID]
	if rec == nil {
		return
	}
	rec.status = turnFailed
	rec.updatedAt = time.Now()
	rec.wait.resetResumeClaim()
	rec.wait.markDone(rawAssistant, visibleAssistant, responseBody, statusCode, true)
}

func writeTurnResponse(w http.ResponseWriter, rec *chatTurnRecord) {
	body, statusCode, failed, ok := rec.wait.snapshot()
	if !ok || len(body) == 0 {
		if failed {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		http.Error(w, "turn not ready", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
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
			if _, ok := s.sessions[sessionID]; !ok {
				delete(s.bootstrap, sessionID)
			}
		}
		s.mu.Unlock()
	}
}
