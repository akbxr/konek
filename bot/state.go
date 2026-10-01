package bot

import (
	"sync"
)

type SessionState struct {
	mu                sync.RWMutex
	SelectedAgent     map[int64]string // userID -> paneID
	SelectedWorkspace map[int64]string // userID -> workspaceID
}

func NewSessionState() *SessionState {
	return &SessionState{
		SelectedAgent:     make(map[int64]string),
		SelectedWorkspace: make(map[int64]string),
	}
}

func (s *SessionState) GetSelectedAgent(userID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SelectedAgent[userID]
}

func (s *SessionState) SetSelectedAgent(userID int64, paneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedAgent[userID] = paneID
}

func (s *SessionState) GetSelectedWorkspace(userID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SelectedWorkspace[userID]
}

func (s *SessionState) SetSelectedWorkspace(userID int64, wsID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedWorkspace[userID] = wsID
}
