package directdeploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitopshq-io/agent/internal/adapter/render"
	"github.com/gitopshq-io/agent/internal/domain"
)

type fakeRuntime struct {
	applied   []render.Manifest
	namespace string
	scaled    *domain.ScaleWorkloadCommand
}

func (r *fakeRuntime) ApplyRendered(_ context.Context, namespace string, manifests []render.Manifest) ([]domain.ResourceRef, error) {
	r.namespace = namespace
	r.applied = append(r.applied, manifests...)
	refs := make([]domain.ResourceRef, 0, len(manifests))
	for _, manifest := range manifests {
		refs = append(refs, domain.ResourceRef{Kind: manifest.Kind, Name: manifest.Name})
	}
	return refs, nil
}

func (r *fakeRuntime) RestartWorkload(context.Context, domain.RestartWorkloadCommand) error {
	return nil
}

func (r *fakeRuntime) ScaleWorkload(_ context.Context, command domain.ScaleWorkloadCommand) error {
	r.scaled = &command
	return nil
}

func (r *fakeRuntime) CollectDrift(context.Context) (*domain.DriftReport, error) {
	return &domain.DriftReport{Timestamp: time.Now().UTC()}, nil
}

type fakeSourceLoader struct {
	root   string
	values map[string]any
}

func (l fakeSourceLoader) CheckoutGit(context.Context, domain.SourceRef) (string, func(), error) {
	return l.root, func() {}, nil
}

func (l fakeSourceLoader) PullHelmOCI(context.Context, domain.SourceRef) ([]byte, error) {
	return nil, nil
}

func (l fakeSourceLoader) ResolveValues(context.Context, *domain.ValuesRef) (map[string]any, error) {
	return l.values, nil
}

func TestExecutorApplyManifestBundle(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bundle.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
---
apiVersion: v1
kind: Service
metadata:
  name: api
`), 0o600); err != nil {
		t.Fatalf("write manifest bundle: %v", err)
	}

	runtime := &fakeRuntime{}
	executor := Executor{
		Runtime: runtime,
		Sources: fakeSourceLoader{root: root},
	}

	result, err := executor.Execute(context.Background(), domain.ExecuteCommand{
		CommandID: "cmd-1",
		ApplyManifestBundle: &domain.ApplyManifestBundleCommand{
			Namespace: "payments",
			Source: domain.SourceRef{
				Type:             "manifest_git",
				URL:              "https://example.com/repo.git",
				ResolvedRevision: "deadbeef",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != domain.CommandStatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if runtime.namespace != "payments" {
		t.Fatalf("expected namespace %q, got %q", "payments", runtime.namespace)
	}
	if len(runtime.applied) != 2 {
		t.Fatalf("expected 2 rendered manifests, got %d", len(runtime.applied))
	}
}

func TestExecutorScaleWorkload(t *testing.T) {
	runtime := &fakeRuntime{}
	executor := Executor{
		Runtime: runtime,
		Sources: fakeSourceLoader{},
	}

	result, err := executor.Execute(context.Background(), domain.ExecuteCommand{
		CommandID: "cmd-1",
		ScaleWorkload: &domain.ScaleWorkloadCommand{
			Namespace: "payments",
			Kind:      "Deployment",
			Name:      "api",
			Replicas:  5,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != domain.CommandStatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
	if runtime.scaled == nil || runtime.scaled.Replicas != 5 {
		t.Fatalf("expected scale command to reach runtime, got %#v", runtime.scaled)
	}
}

func TestExecutorRejectsManifestSourcePathEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: outside\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor := Executor{
		Runtime: &fakeRuntime{},
		Sources: fakeSourceLoader{root: root},
	}

	_, err := executor.Execute(context.Background(), domain.ExecuteCommand{
		CommandID: "cmd-escape",
		ApplyManifestBundle: &domain.ApplyManifestBundleCommand{
			Namespace: "payments",
			Source: domain.SourceRef{
				Type:             "manifest_git",
				URL:              "https://example.com/repo.git",
				ResolvedRevision: "deadbeef",
				Path:             "../outside.yaml",
			},
		},
	})
	if err == nil {
		t.Fatal("expected repository path escape to be rejected")
	}
}
