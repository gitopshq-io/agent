package runtime

import (
	"context"
	"testing"

	"github.com/gitopshq-io/agent/internal/domain"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestSecretIdentityStoreSaveAndLoad(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	store := SecretIdentityStore{
		Client:     clientset,
		Namespace:  "gitopshq-system",
		SecretName: "gitopshq-agent-identity",
	}
	want := domain.AgentIdentity{
		AgentToken: "ghqa_example",
		ClusterID:  "cluster-1",
	}
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestSecretIdentityStoreUpdatesExistingSecret(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	store := SecretIdentityStore{
		Client:     clientset,
		Namespace:  "gitopshq-system",
		SecretName: "gitopshq-agent-identity",
	}
	if err := store.Save(context.Background(), domain.AgentIdentity{AgentToken: "old", ClusterID: "cluster-1"}); err != nil {
		t.Fatalf("Save(initial) error = %v", err)
	}
	if err := store.Save(context.Background(), domain.AgentIdentity{AgentToken: "new", ClusterID: "cluster-2"}); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.AgentToken != "new" || got.ClusterID != "cluster-2" {
		t.Fatalf("Load() = %#v, want updated identity", got)
	}
}
