package usecase

import (
	"sync"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

const defaultStatusInterval = 30 * time.Second

type SessionSnapshot struct {
	ClusterID      string
	AgentVersion   string
	StatusInterval time.Duration
	Capabilities   []domain.Capability
}

type SessionState struct {
	mu             sync.RWMutex
	clusterID      string
	agentVersion   string
	statusInterval time.Duration
	capabilities   domain.CapabilitySet
}

func NewSessionState(clusterID, agentVersion string, statusInterval time.Duration, capabilities []domain.Capability) *SessionState {
	state := &SessionState{
		clusterID:      clusterID,
		agentVersion:   agentVersion,
		statusInterval: statusInterval,
		capabilities:   domain.NewCapabilitySet(capabilities),
	}
	if state.statusInterval <= 0 {
		state.statusInterval = defaultStatusInterval
	}
	return state
}

func (s *SessionState) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionSnapshot{
		ClusterID:      s.clusterID,
		AgentVersion:   s.agentVersion,
		StatusInterval: s.statusInterval,
		Capabilities:   append([]domain.Capability(nil), s.capabilities.Values()...),
	}
}

func (s *SessionState) Capabilities() domain.CapabilitySet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(domain.CapabilitySet, len(s.capabilities))
	for capability := range s.capabilities {
		out[capability] = struct{}{}
	}
	return out
}

func (s *SessionState) SetClusterID(clusterID string) {
	if clusterID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterID = clusterID
}

func (s *SessionState) ApplyConfigUpdate(update domain.ConfigUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if update.StatusInterval > 0 {
		s.statusInterval = update.StatusInterval
	}
	if len(update.Capabilities) > 0 {
		s.capabilities = domain.NewCapabilitySet(update.Capabilities)
	}
	if s.statusInterval <= 0 {
		s.statusInterval = defaultStatusInterval
	}
}
