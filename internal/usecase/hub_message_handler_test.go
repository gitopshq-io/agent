package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

type fakeIdentityStore struct {
	identity domain.AgentIdentity
}

func (s *fakeIdentityStore) Load(context.Context) (domain.AgentIdentity, error) {
	return s.identity, nil
}

func (s *fakeIdentityStore) Save(_ context.Context, identity domain.AgentIdentity) error {
	s.identity = identity
	return nil
}

type fakeSyncer struct {
	result domain.CredentialSyncResult
}

func (s fakeSyncer) Sync(context.Context, domain.CredentialSyncRequest) (domain.CredentialSyncResult, error) {
	return s.result, nil
}

func TestHubMessageHandlerConfigUpdateAndPing(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	state := NewSessionState("cluster-1", "1.0.0", 30*time.Second, []domain.Capability{domain.CapabilityObserve})
	handler := HubMessageHandler{
		Store:          &fakeIdentityStore{},
		Executor:       ExecuteCommand{Executor: &fakeExecutor{}, Clock: clock},
		CredentialSync: SyncCredentials{Syncer: fakeSyncer{}, Clock: clock},
		State:          state,
		Clock:          clock,
	}

	if _, err := handler.Handle(context.Background(), domain.HubMessage{
		ConfigUpdate: &domain.ConfigUpdate{
			StatusInterval: time.Minute,
			Capabilities:   []domain.Capability{domain.CapabilityObserve, domain.CapabilityCredentialSync},
		},
	}); err != nil {
		t.Fatalf("Handle(config update) error = %v", err)
	}

	messages, err := handler.Handle(context.Background(), domain.HubMessage{Ping: &domain.Ping{Timestamp: clock.current}})
	if err != nil {
		t.Fatalf("Handle(ping) error = %v", err)
	}
	if len(messages) != 1 || messages[0].Heartbeat == nil {
		t.Fatalf("expected heartbeat response, got %#v", messages)
	}
	if messages[0].Heartbeat.ClusterID != "cluster-1" {
		t.Fatalf("expected cluster id to be preserved, got %q", messages[0].Heartbeat.ClusterID)
	}
	if !state.Capabilities().Has(domain.CapabilityCredentialSync) {
		t.Fatal("expected config update to replace capabilities")
	}
}

func TestHubMessageHandlerRotateTokenPersistsIdentity(t *testing.T) {
	store := &fakeIdentityStore{identity: domain.AgentIdentity{AgentToken: "old-token", ClusterID: "cluster-1"}}
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	state := NewSessionState("cluster-1", "1.0.0", 30*time.Second, []domain.Capability{domain.CapabilityTokenRotate})
	handler := HubMessageHandler{
		Store:          store,
		Executor:       ExecuteCommand{Executor: &fakeExecutor{}, Clock: clock},
		CredentialSync: SyncCredentials{Syncer: fakeSyncer{}, Clock: clock},
		State:          state,
		Clock:          clock,
	}

	messages, err := handler.Handle(context.Background(), domain.HubMessage{
		RotateToken: &domain.RotateToken{
			CommandID: "cmd-rotate",
			NewToken:  "new-token",
		},
	})
	if err != nil {
		t.Fatalf("Handle(rotate token) error = %v", err)
	}
	if store.identity.AgentToken != "new-token" {
		t.Fatalf("expected token to be updated, got %q", store.identity.AgentToken)
	}
	if len(messages) != 1 || messages[0].CommandResult == nil {
		t.Fatalf("expected command result, got %#v", messages)
	}
	if messages[0].CommandResult.Status != domain.CommandStatusCompleted {
		t.Fatalf("expected completed status, got %q", messages[0].CommandResult.Status)
	}
}
