package bot

import (
	"context"
	"sync"
	"time"
)

type ActiveJob struct {
	PaneID    string
	AgentName string
	Prompt    string
	StartTime time.Time
	Cancel    context.CancelFunc
}

type SessionState struct {
	mu                sync.RWMutex
	SelectedAgent     map[int64]string // userID -> paneID
	SelectedWorkspace map[int64]string // userID -> workspaceID
	ActiveJobs        map[string]*ActiveJob
}

func NewSessionState() *SessionState {
	return &SessionState{
		SelectedAgent:     make(map[int64]string),
		SelectedWorkspace: make(map[int64]string),
		ActiveJobs:        make(map[string]*ActiveJob),
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

func (s *SessionState) AddJob(paneID, agentName, prompt string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveJobs[paneID] = &ActiveJob{
		PaneID:    paneID,
		AgentName: agentName,
		Prompt:    prompt,
		StartTime: time.Now(),
		Cancel:    cancel,
	}
}

func (s *SessionState) AbortJob(paneID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.ActiveJobs[paneID]; ok {
		if job.Cancel != nil {
			job.Cancel()
		}
		delete(s.ActiveJobs, paneID)
		return true
	}
	return false
}
func (s *SessionState) RemoveJob(paneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ActiveJobs, paneID)
}

func (s *SessionState) GetActiveJobs() []*ActiveJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var jobs []*ActiveJob
	for _, j := range s.ActiveJobs {
		jobs = append(jobs, j)
	}
	return jobs
}
