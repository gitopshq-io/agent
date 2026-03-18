package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
	cfgpkg "github.com/gitopshq-io/agent/internal/platform/config"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(cfg cfgpkg.ArgoCDConfig) *Client {
	if cfg.ServerURL == "" {
		return nil
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.ServerURL, "/"),
		token:   cfg.Token,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure}, //nolint:gosec
			},
		},
	}
}

func (c *Client) CollectApplications(ctx context.Context) (*domain.ArgoApplicationStatus, error) {
	if c == nil {
		return &domain.ArgoApplicationStatus{Timestamp: time.Now().UTC()}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/applications", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return nil, fmt.Errorf("read argocd applications response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("argocd list applications returned %s: %s", resp.Status, compactBody(body))
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Project string `json:"project"`
				Source  struct {
					RepoURL        string `json:"repoURL"`
					Path           string `json:"path"`
					TargetRevision string `json:"targetRevision"`
				} `json:"source"`
			} `json:"spec"`
			Status struct {
				Sync struct {
					Status string `json:"status"`
				} `json:"sync"`
				Health struct {
					Status string `json:"status"`
				} `json:"health"`
				OperationState struct {
					FinishedAt string `json:"finishedAt"`
				} `json:"operationState"`
				Resources []any `json:"resources"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	out := &domain.ArgoApplicationStatus{Timestamp: time.Now().UTC()}
	for _, item := range payload.Items {
		app := domain.ArgoApplication{
			Name:           item.Metadata.Name,
			Namespace:      item.Metadata.Namespace,
			Project:        item.Spec.Project,
			RepoURL:        item.Spec.Source.RepoURL,
			Path:           item.Spec.Source.Path,
			TargetRevision: item.Spec.Source.TargetRevision,
			SyncStatus:     item.Status.Sync.Status,
			HealthStatus:   item.Status.Health.Status,
			ResourceCount:  len(item.Status.Resources),
		}
		if item.Status.OperationState.FinishedAt != "" {
			if ts, err := time.Parse(time.RFC3339, item.Status.OperationState.FinishedAt); err == nil {
				app.LastSyncedAt = ts.UTC()
			}
		}
		out.Applications = append(out.Applications, app)
	}
	return out, nil
}

func (c *Client) Execute(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	if c == nil {
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     "argocd is not configured",
			Timestamp: time.Now().UTC(),
		}, nil
	}
	switch {
	case cmd.ArgoSync != nil:
		body, _ := json.Marshal(map[string]any{
			"prune":  cmd.ArgoSync.Prune,
			"dryRun": cmd.ArgoSync.DryRun,
		})
		return c.do(ctx, cmd.CommandID, http.MethodPost, "/api/v1/applications/"+cmd.ArgoSync.Application+"/sync", body)
	case cmd.ArgoRollback != nil:
		body, _ := json.Marshal(map[string]any{
			"id":    cmd.ArgoRollback.ID,
			"prune": cmd.ArgoRollback.Prune,
		})
		return c.do(ctx, cmd.CommandID, http.MethodPost, "/api/v1/applications/"+cmd.ArgoRollback.Application+"/rollback", body)
	default:
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     "argocd adapter cannot execute this command",
			Timestamp: time.Now().UTC(),
		}, nil
	}
}

func (c *Client) do(ctx context.Context, commandID, method, path string, body []byte) (domain.CommandResult, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return domain.CommandResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.CommandResult{}, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode >= 300 {
		return domain.CommandResult{
			CommandID: commandID,
			Status:    domain.CommandStatusFailed,
			Error:     fmt.Sprintf("%s: %s", resp.Status, compactBody(bodyBytes)),
			Timestamp: time.Now().UTC(),
		}, nil
	}
	return domain.CommandResult{
		CommandID: commandID,
		Status:    domain.CommandStatusCompleted,
		Message:   "command executed by argocd adapter",
		Timestamp: time.Now().UTC(),
	}, nil
}

func compactBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "<empty body>"
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 256 {
		return text[:256] + "..."
	}
	return text
}
