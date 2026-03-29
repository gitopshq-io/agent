package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

func TestFileIdentityStoreSaveCreatesParentDirectories(t *testing.T) {
	store := FileIdentityStore{
		Path: filepath.Join(t.TempDir(), "nested", "agent", "identity.json"),
	}
	identity := domain.AgentIdentity{
		AgentToken: "agent-token",
		ClusterID:  "cluster-1",
	}

	if err := store.Save(context.Background(), identity); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != identity {
		t.Fatalf("expected %#v, got %#v", identity, got)
	}
}

type fakeCommandExecutor struct {
	called bool
	last   domain.ExecuteCommand
	result domain.CommandResult
	err    error
}

func (f *fakeCommandExecutor) Execute(_ context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	f.called = true
	f.last = cmd
	if f.err != nil {
		return domain.CommandResult{}, f.err
	}
	return f.result, nil
}

func TestCommandRouterRoutesArgoDeleteToArgoExecutor(t *testing.T) {
	argo := &fakeCommandExecutor{
		result: domain.CommandResult{
			CommandID: "cmd-delete",
			Status:    domain.CommandStatusCompleted,
			Timestamp: time.Now().UTC(),
		},
	}
	direct := &fakeCommandExecutor{}
	router := CommandRouter{
		Argo:   argo,
		Direct: direct,
	}
	cmd := domain.ExecuteCommand{
		CommandID:  "cmd-delete",
		ArgoDelete: &domain.ArgoDeleteCommand{Application: "payments"},
	}

	result, err := router.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !argo.called {
		t.Fatal("expected Argo executor to be called")
	}
	if direct.called {
		t.Fatal("expected Direct executor not to be called")
	}
	if result.Status != domain.CommandStatusCompleted {
		t.Fatalf("result.Status = %q, want %q", result.Status, domain.CommandStatusCompleted)
	}
}
