// Package goal provides session-scoped goal state management for the
// autonomous /goal feature. A goal is a natural-language completion
// condition that the agent works toward across multiple turns.
package goal

import (
	"sync"
	"time"
)

// MaxGoalTurns is the safety limit on autonomous continuation turns.
const MaxGoalTurns = 50

// State represents the lifecycle of a single goal on a session.
type State struct {
	Condition   string
	Active      bool
	Paused      bool
	StartedAt   time.Time
	TurnCount   int
	CompletedAt time.Time
	LastReason  string
}

// Service is a thread-safe, in-memory store of goal state keyed by
// session ID. One goal per session — setting a new goal replaces any
// existing one.
type Service struct {
	mu    sync.RWMutex
	goals map[string]*State
}

// NewService creates an empty goal service.
func NewService() *Service {
	return &Service{
		goals: make(map[string]*State),
	}
}

// Set activates a goal for the given session, replacing any prior goal.
func (s *Service) Set(sessionID, condition string) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &State{
		Condition: condition,
		Active:    true,
		StartedAt: time.Now(),
	}
	s.goals[sessionID] = st
	return st
}

// Get returns the current goal state for the session, or nil if none.
func (s *Service) Get(sessionID string) *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.goals[sessionID]
}

// Clear removes the goal for the session.
func (s *Service) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.goals, sessionID)
}

// Pause marks the session's goal as paused (auto-continue skips paused goals).
func (s *Service) Pause(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.goals[sessionID]; ok {
		st.Paused = true
	}
}

// Resume clears the paused flag on the session's goal.
func (s *Service) Resume(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.goals[sessionID]; ok {
		st.Paused = false
	}
}

// IncrementTurn increments the turn counter and returns the new value.
// No-op if no goal exists for the session.
func (s *Service) IncrementTurn(sessionID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.goals[sessionID]; ok {
		st.TurnCount++
		return st.TurnCount
	}
	return 0
}

// Complete marks the goal as completed and records the evaluator's reason.
func (s *Service) Complete(sessionID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.goals[sessionID]; ok {
		st.Active = false
		st.CompletedAt = time.Now()
		st.LastReason = reason
	}
}

// SetReason updates the last evaluator reason without changing lifecycle state.
func (s *Service) SetReason(sessionID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.goals[sessionID]; ok {
		st.LastReason = reason
	}
}
