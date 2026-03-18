package runtime

import (
	"context"
	"path/filepath"
	"testing"

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
