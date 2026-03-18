package usecase

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type fakeInventoryCollector struct {
	snapshot *domain.InventorySnapshot
	drift    *domain.DriftReport
}

func (c fakeInventoryCollector) CollectInventory(context.Context) (*domain.InventorySnapshot, error) {
	return c.snapshot, nil
}

func (c fakeInventoryCollector) CollectDrift(context.Context) (*domain.DriftReport, error) {
	return c.drift, nil
}

type fakeAppObserver struct {
	status *domain.ArgoApplicationStatus
}

func (o fakeAppObserver) CollectApplications(context.Context) (*domain.ArgoApplicationStatus, error) {
	return o.status, nil
}

type fakeHubSession struct {
	mu       sync.Mutex
	sent     []domain.AgentMessage
	recvChan chan domain.HubMessage
}

func (s *fakeHubSession) Send(msg domain.AgentMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, msg)
	return nil
}

func (s *fakeHubSession) Recv() (domain.HubMessage, error) {
	msg, ok := <-s.recvChan
	if !ok {
		return domain.HubMessage{}, io.EOF
	}
	return msg, nil
}

func (s *fakeHubSession) CloseSend() error {
	return nil
}

func (s *fakeHubSession) Sent() []domain.AgentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.AgentMessage(nil), s.sent...)
}

type fakeHubTransport struct {
	session port.HubSession
}

func (t fakeHubTransport) Register(context.Context, string, domain.Cluster) (domain.RegisterResponse, error) {
	return domain.RegisterResponse{}, nil
}

func (t fakeHubTransport) Connect(context.Context, string) (port.HubSession, error) {
	return t.session, nil
}

func TestMaintainSessionRunSendsInitialStatusWithClusterIdentity(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	session := &fakeHubSession{
		recvChan: make(chan domain.HubMessage, 1),
	}
	session.recvChan <- domain.HubMessage{Ping: &domain.Ping{Timestamp: clock.current}}
	close(session.recvChan)

	store := &fakeIdentityStore{identity: domain.AgentIdentity{AgentToken: "agent-token", ClusterID: "cluster-1"}}
	state := NewSessionState("", "1.0.0", time.Hour, []domain.Capability{domain.CapabilityObserve})
	runner := MaintainSession{
		Transport: fakeHubTransport{session: session},
		Store:     store,
		Reporter: SessionReporter{
			Inventory: CollectInventory{
				Collector: fakeInventoryCollector{
					snapshot: &domain.InventorySnapshot{Summary: domain.InventorySummary{ClusterName: "cluster-a"}},
					drift:    &domain.DriftReport{},
				},
				Clock: clock,
			},
			Drift: ReportDrift{
				Collector: fakeInventoryCollector{
					snapshot: &domain.InventorySnapshot{},
					drift:    &domain.DriftReport{},
				},
				Clock: clock,
			},
			Applications: fakeAppObserver{status: &domain.ArgoApplicationStatus{}},
			Clock:        clock,
		},
		Handler: HubMessageHandler{
			Store:          store,
			Executor:       ExecuteCommand{Executor: &fakeExecutor{}, Clock: clock},
			CredentialSync: SyncCredentials{Syncer: fakeSyncer{}, Clock: clock},
			State:          state,
			Clock:          clock,
		},
		State: state,
	}

	if err := runner.Run(context.Background()); !errors.Is(err, errSessionClosed) {
		t.Fatalf("Run() error = %v, want %v", err, errSessionClosed)
	}

	sent := session.Sent()
	if len(sent) == 0 {
		t.Fatal("expected outbound messages to be sent")
	}
	if sent[0].Heartbeat == nil {
		t.Fatalf("expected first outbound message to be heartbeat, got %#v", sent[0])
	}
	if sent[0].Heartbeat.ClusterID != "cluster-1" {
		t.Fatalf("expected cluster id %q, got %q", "cluster-1", sent[0].Heartbeat.ClusterID)
	}
}
