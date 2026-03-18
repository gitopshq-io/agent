package kubernetes

import (
	"context"
	"testing"

	"github.com/gitopshq-io/agent/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestCollectInventoryReportsKubernetesVersion(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	discoveryClient, ok := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatalf("expected fake discovery client, got %T", clientset.Discovery())
	}
	discoveryClient.FakedServerVersion = &version.Info{GitVersion: "v1.32.1"}

	client := NewWithClients(clientset, nil, nil, "agent-system", "gitopshq-agent", nil)
	snapshot, err := client.CollectInventory(context.Background())
	if err != nil {
		t.Fatalf("CollectInventory() error = %v", err)
	}
	if snapshot.Summary.KubernetesVersion != "v1.32.1" {
		t.Fatalf("expected kubernetes version %q, got %q", "v1.32.1", snapshot.Summary.KubernetesVersion)
	}
}

func TestMirrorCredentialsUpsertsAndPrunesManagedSecrets(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stale",
				Namespace: "team-a",
				Labels: map[string]string{
					managedByLabelKey: managedByLabelValue,
				},
			},
		},
	)
	client := NewWithClients(clientset, nil, nil, "agent-system", "gitopshq-agent", nil)

	result, err := client.MirrorCredentials(context.Background(), domain.CredentialSyncRequest{
		Version: "v2",
		Bundles: []domain.CredentialBundle{
			{
				Namespace:  "team-a",
				SecretName: "git-creds",
				StringData: map[string]string{"username": "x-token", "password": "abc"},
			},
		},
	}, []string{"team-a"})
	if err != nil {
		t.Fatalf("MirrorCredentials() error = %v", err)
	}
	if result.Status != "synced" {
		t.Fatalf("expected synced status, got %q", result.Status)
	}

	secret, err := clientset.CoreV1().Secrets("team-a").Get(context.Background(), "git-creds", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get mirrored secret: %v", err)
	}
	if secret.Labels[managedByLabelKey] != managedByLabelValue {
		t.Fatalf("expected managed-by label, got %#v", secret.Labels)
	}
	if secret.Labels[versionLabelKey] != "v2" {
		t.Fatalf("expected version label, got %#v", secret.Labels)
	}
	if _, err := clientset.CoreV1().Secrets("team-a").Get(context.Background(), "stale", metav1.GetOptions{}); err == nil {
		t.Fatal("expected stale managed secret to be deleted")
	}
}
