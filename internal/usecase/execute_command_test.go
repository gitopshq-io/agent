package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

type fakeClock struct {
	current time.Time
}

func (c fakeClock) Now() time.Time {
	return c.current
}

type fakeExecutor struct {
	called bool
	last   domain.ExecuteCommand
	result domain.CommandResult
}

func (e *fakeExecutor) Execute(_ context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	e.called = true
	e.last = cmd
	return e.result, nil
}

func TestExecuteCommandRunRejectsMissingCapability(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	executor := &fakeExecutor{}
	command := domain.ExecuteCommand{
		CommandID: "cmd-1",
		ExpiresAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:  &domain.ArgoSyncCommand{Application: "payments"},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}

	result, err := ExecuteCommand{Executor: executor, Clock: clock}.Run(context.Background(), command, domain.NewCapabilitySet([]domain.Capability{domain.CapabilityObserve}))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != domain.CommandStatusFailed {
		t.Fatalf("expected failed status, got %q", result.Status)
	}
	if executor.called {
		t.Fatal("expected executor not to be called")
	}
}

func TestExecuteCommandRunExecutesAllowedCommand(t *testing.T) {
	clock := fakeClock{current: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)}
	executor := &fakeExecutor{
		result: domain.CommandResult{
			Status:  domain.CommandStatusCompleted,
			Message: "ok",
		},
	}
	command := domain.ExecuteCommand{
		CommandID: "cmd-1",
		ExpiresAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:  &domain.ArgoSyncCommand{Application: "payments"},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}

	result, err := ExecuteCommand{Executor: executor, Clock: clock}.Run(context.Background(), command, domain.NewCapabilitySet([]domain.Capability{domain.CapabilityArgoCDWrite}))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !executor.called {
		t.Fatal("expected executor to be called")
	}
	if result.Status != domain.CommandStatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if result.CommandID != command.CommandID {
		t.Fatalf("expected command id %q, got %q", command.CommandID, result.CommandID)
	}
	if result.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be populated")
	}
}
