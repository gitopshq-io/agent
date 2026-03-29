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

type fakeApplicationObserverForHandler struct {
	status *domain.ArgoApplicationStatus
	err    error
	calls  int
}

func (o *fakeApplicationObserverForHandler) CollectApplications(context.Context) (*domain.ArgoApplicationStatus, error) {
	o.calls++
	if o.err != nil {
		return nil, o.err
	}
	return o.status, nil
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

func TestHubMessageHandlerExecuteCommandPublishesApplicationStatusForArgoMutations(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	state := NewSessionState("cluster-1", "1.0.0", 30*time.Second, []domain.Capability{domain.CapabilityArgoCDWrite})
	executor := &fakeExecutor{
		result: domain.CommandResult{
			Status:  domain.CommandStatusCompleted,
			Message: "ok",
		},
	}
	appObserver := &fakeApplicationObserverForHandler{
		status: &domain.ArgoApplicationStatus{
			Applications: []domain.ArgoApplication{
				{Name: "payments", Namespace: "devops"},
			},
		},
	}
	handler := HubMessageHandler{
		Store:          &fakeIdentityStore{},
		Executor:       ExecuteCommand{Executor: executor, Clock: clock},
		CredentialSync: SyncCredentials{Syncer: fakeSyncer{}, Clock: clock},
		Applications:   appObserver,
		State:          state,
		Clock:          clock,
	}
	command := domain.ExecuteCommand{
		CommandID: "cmd-sync",
		ExpiresAt: clock.current.Add(30 * time.Second),
		ArgoSync: &domain.ArgoSyncCommand{
			Application: "payments",
			Namespace:   "devops",
		},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}

	messages, err := handler.Handle(context.Background(), domain.HubMessage{
		ExecuteCommand: &command,
	})
	if err != nil {
		t.Fatalf("Handle(execute command) error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected ack + result + application status, got %#v", messages)
	}
	if messages[0].CommandAck == nil || messages[0].CommandAck.Status != domain.CommandStatusAcked {
		t.Fatalf("expected accepted command ack, got %#v", messages[0])
	}
	if messages[1].CommandResult == nil || messages[1].CommandResult.Status != domain.CommandStatusCompleted {
		t.Fatalf("expected completed command result, got %#v", messages[1])
	}
	if messages[2].ApplicationStatus == nil {
		t.Fatalf("expected immediate application status message, got %#v", messages[2])
	}
	if messages[2].ApplicationStatus.Timestamp.IsZero() {
		t.Fatal("expected application status timestamp to be populated")
	}
	if !messages[2].ApplicationStatus.Timestamp.Equal(clock.current) {
		t.Fatalf("expected timestamp %s, got %s", clock.current, messages[2].ApplicationStatus.Timestamp)
	}
	if appObserver.calls != 1 {
		t.Fatalf("expected app observer to be called once, got %d", appObserver.calls)
	}
}

func TestHubMessageHandlerExecuteCommandSkipsApplicationStatusForNonArgoCommand(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	state := NewSessionState("cluster-1", "1.0.0", 30*time.Second, []domain.Capability{domain.CapabilityDirectDeploy})
	executor := &fakeExecutor{
		result: domain.CommandResult{
			Status:  domain.CommandStatusCompleted,
			Message: "ok",
		},
	}
	appObserver := &fakeApplicationObserverForHandler{
		status: &domain.ArgoApplicationStatus{
			Applications: []domain.ArgoApplication{{Name: "payments"}},
		},
	}
	handler := HubMessageHandler{
		Store:          &fakeIdentityStore{},
		Executor:       ExecuteCommand{Executor: executor, Clock: clock},
		CredentialSync: SyncCredentials{Syncer: fakeSyncer{}, Clock: clock},
		Applications:   appObserver,
		State:          state,
		Clock:          clock,
	}
	command := domain.ExecuteCommand{
		CommandID: "cmd-apply",
		ExpiresAt: clock.current.Add(30 * time.Second),
		ApplyManifestBundle: &domain.ApplyManifestBundleCommand{
			Namespace: "default",
			Source: domain.SourceRef{
				Type: "manifest_git",
				URL:  "https://example.com/repo.git",
				Path: ".argocd",
			},
		},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}

	messages, err := handler.Handle(context.Background(), domain.HubMessage{
		ExecuteCommand: &command,
	})
	if err != nil {
		t.Fatalf("Handle(execute command) error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected ack + result only, got %#v", messages)
	}
	for _, message := range messages {
		if message.ApplicationStatus != nil {
			t.Fatalf("did not expect application status for non-argo command, got %#v", message)
		}
	}
	if appObserver.calls != 0 {
		t.Fatalf("expected app observer to not be called, got %d", appObserver.calls)
	}
}
