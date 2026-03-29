package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gitopshq-io/agent/internal/domain"
)

type fakeSecretReader struct {
	data map[string][]byte
}

func (r fakeSecretReader) ReadSecretData(context.Context, domain.CredentialRef) (map[string][]byte, error) {
	return r.data, nil
}

func TestResolveValuesRejectsCredentialBackedValuesWithoutDigest(t *testing.T) {
	loader := Loader{
		Secrets: fakeSecretReader{
			data: map[string][]byte{"values.yaml": []byte("replicaCount: 2")},
		},
	}

	_, err := loader.ResolveValues(context.Background(), &domain.ValuesRef{
		CredentialRef: &domain.CredentialRef{
			Namespace:  "team-a",
			SecretName: "values-secret",
		},
	})
	if err == nil {
		t.Fatal("expected missing digest to be rejected")
	}
}

func TestResolveValuesRejectsDigestMismatch(t *testing.T) {
	loader := Loader{
		Secrets: fakeSecretReader{
			data: map[string][]byte{"values.yaml": []byte("replicaCount: 2")},
		},
	}

	_, err := loader.ResolveValues(context.Background(), &domain.ValuesRef{
		Digest: "sha256:deadbeef",
		CredentialRef: &domain.CredentialRef{
			Namespace:  "team-a",
			SecretName: "values-secret",
		},
	})
	if err == nil {
		t.Fatal("expected digest mismatch to be rejected")
	}
}

func TestResolvePathRejectsEscapeOutsideRepositoryRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if _, err := ResolvePath(root, "../outside"); err == nil {
		t.Fatal("expected source path escape to be rejected")
	}
}

func TestLoadFilesRejectsSymlinkedEntries(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "payload.yaml")
	if err := os.WriteFile(outside, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: outside\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "bundle.yaml")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := LoadFiles(root); err == nil {
		t.Fatal("expected symlinked source entry to be rejected")
	}
}

func TestLoadFilesFromSingleFileUsesBaseFilename(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "single.yaml")
	payload := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: one\n")
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	files, err := LoadFiles(filePath)
	if err != nil {
		t.Fatalf("LoadFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("LoadFiles() returned %d files, want 1", len(files))
	}
	content, ok := files["single.yaml"]
	if !ok {
		t.Fatalf("LoadFiles() key = %#v, want single.yaml", files)
	}
	if content != string(payload) {
		t.Fatalf("LoadFiles() content mismatch, got %q", content)
	}
}
