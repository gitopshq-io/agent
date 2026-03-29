package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gitopshq-io/agent/internal/domain"
)

func TestNormalizeServerURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		insecure bool
		want     string
	}{
		{
			name:     "empty",
			raw:      "",
			insecure: false,
			want:     "",
		},
		{
			name:     "keeps explicit https",
			raw:      "https://argo.example.internal/",
			insecure: true,
			want:     "https://argo.example.internal",
		},
		{
			name:     "uses http when insecure and scheme missing",
			raw:      "argo-cd-server.devops.svc.cluster.local",
			insecure: true,
			want:     "http://argo-cd-server.devops.svc.cluster.local",
		},
		{
			name:     "uses https when secure and scheme missing",
			raw:      "argo.example.internal",
			insecure: false,
			want:     "https://argo.example.internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeServerURL(tt.raw, tt.insecure); got != tt.want {
				t.Fatalf("NormalizeServerURL(%q, %t) = %q, want %q", tt.raw, tt.insecure, got, tt.want)
			}
		})
	}
}

func TestCollectApplicationsSupportsLargeResponses(t *testing.T) {
	apps := make([]map[string]any, 0, 256)
	for i := 0; i < 256; i++ {
		apps = append(apps, map[string]any{
			"metadata": map[string]any{
				"name":      strings.Repeat("app-", 8) + string(rune('a'+(i%26))) + strings.Repeat("x", 24),
				"namespace": "devops",
			},
			"spec": map[string]any{
				"project": "default",
				"source": map[string]any{
					"repoURL":        "https://github.com/example/very-large-repository.git",
					"path":           "services/" + strings.Repeat("backend/", 4),
					"targetRevision": "main",
				},
			},
			"status": map[string]any{
				"sync": map[string]any{
					"status": "Synced",
				},
				"health": map[string]any{
					"status": "Healthy",
				},
				"resources": make([]any, 16),
			},
		})
	}
	payload := map[string]any{"items": apps}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "test-token",
		http:    server.Client(),
	}

	got, err := client.CollectApplications(context.Background())
	if err != nil {
		t.Fatalf("CollectApplications() error = %v", err)
	}
	if got == nil {
		t.Fatal("CollectApplications() returned nil status")
	}
	if len(got.Applications) != len(apps) {
		t.Fatalf("CollectApplications() returned %d apps, want %d", len(got.Applications), len(apps))
	}
	if got.Applications[0].SyncStatus != "Synced" || got.Applications[0].HealthStatus != "Healthy" {
		t.Fatalf("CollectApplications() returned unexpected first app: %#v", got.Applications[0])
	}
}

func TestCollectApplicationsFailsWhenResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[`))
		_, _ = w.Write([]byte(strings.Repeat(`{"metadata":{"name":"app","namespace":"devops"},"spec":{"project":"default","source":{"repoURL":"https://example.com/repo.git","path":"svc","targetRevision":"main"}},"status":{"sync":{"status":"Synced"},"health":{"status":"Healthy"},"resources":[1,2,3,4,5,6,7,8,9,10]}},`, 50000)))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "test-token",
		http:    server.Client(),
	}

	_, err := client.CollectApplications(context.Background())
	if err == nil {
		t.Fatal("CollectApplications() error = nil, want size limit error")
	}
	if !strings.Contains(err.Error(), "response exceeded") {
		t.Fatalf("CollectApplications() error = %v, want size limit error", err)
	}
}

func TestExecuteArgoDeleteUsesDeleteEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodDelete; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/api/v1/applications/payments"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("appNamespace"); got != "devops" {
			t.Fatalf("appNamespace query = %q, want %q", got, "devops")
		}
		if got := r.URL.Query().Get("cascade"); got != "true" {
			t.Fatalf("cascade query = %q, want %q", got, "true")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization header = %q, want %q", got, "Bearer test-token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "test-token",
		http:    server.Client(),
	}
	result, err := client.Execute(context.Background(), domain.ExecuteCommand{
		CommandID: "cmd-delete-1",
		ArgoDelete: &domain.ArgoDeleteCommand{
			Application: "payments",
			Namespace:   "devops",
			Cascade:     true,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != domain.CommandStatusCompleted {
		t.Fatalf("result.Status = %q, want %q", result.Status, domain.CommandStatusCompleted)
	}
}
