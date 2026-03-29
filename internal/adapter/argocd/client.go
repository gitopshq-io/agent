package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
	cfgpkg "github.com/gitopshq-io/agent/internal/platform/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxApplicationsResponseBytes = 4 << 20
	maxApplicationEvents         = 40
	maxEventSubjects             = 25
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	kube    kubernetes.Interface
}

type applicationSourcePayload struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	Chart          string `json:"chart"`
	TargetRevision string `json:"targetRevision"`
	Ref            string `json:"ref"`
}

type applicationPayload struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Project     string `json:"project"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		Source     applicationSourcePayload   `json:"source"`
		Sources    []applicationSourcePayload `json:"sources"`
		SyncPolicy struct {
			Automated *struct {
				Prune      bool `json:"prune"`
				SelfHeal   bool `json:"selfHeal"`
				AllowEmpty bool `json:"allowEmpty"`
			} `json:"automated"`
		} `json:"syncPolicy"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		ReconciledAt   string `json:"reconciledAt"`
		OperationState struct {
			Phase      string `json:"phase"`
			Message    string `json:"message"`
			StartedAt  string `json:"startedAt"`
			FinishedAt string `json:"finishedAt"`
		} `json:"operationState"`
		Conditions []struct {
			Type               string `json:"type"`
			Message            string `json:"message"`
			LastTransitionTime string `json:"lastTransitionTime"`
		} `json:"conditions"`
		Resources []struct {
			Group           string `json:"group"`
			Kind            string `json:"kind"`
			Namespace       string `json:"namespace"`
			Name            string `json:"name"`
			Version         string `json:"version"`
			Status          string `json:"status"`
			HookType        string `json:"hookType"`
			RequiresPruning bool   `json:"requiresPruning"`
			Health          *struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"health"`
		} `json:"resources"`
		History []struct {
			ID         int64                      `json:"id"`
			Revision   string                     `json:"revision"`
			DeployedAt string                     `json:"deployedAt"`
			Source     *applicationSourcePayload  `json:"source"`
			Sources    []applicationSourcePayload `json:"sources"`
		} `json:"history"`
	} `json:"status"`
}

func NormalizeServerURL(raw string, insecure bool) string {
	server := strings.TrimSpace(raw)
	if server == "" {
		return ""
	}
	if strings.Contains(server, "://") {
		return strings.TrimRight(server, "/")
	}
	scheme := "https://"
	if insecure {
		scheme = "http://"
	}
	return strings.TrimRight(scheme+server, "/")
}

func New(cfg cfgpkg.ArgoCDConfig, kube kubernetes.Interface) *Client {
	baseURL := NormalizeServerURL(cfg.ServerURL, cfg.Insecure)
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL: baseURL,
		token:   cfg.Token,
		kube:    kube,
		http: &http.Client{
			Timeout: 20 * time.Second,
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

	limited := &io.LimitedReader{R: resp.Body, N: maxApplicationsResponseBytes}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read argocd applications response: %w", err)
	}
	if limited.N == 0 {
		return nil, fmt.Errorf("argocd list applications response exceeded %d bytes", maxApplicationsResponseBytes)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("argocd list applications returned %s: %s", resp.Status, compactBody(body))
	}

	var payload struct {
		Items []applicationPayload `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	out := &domain.ArgoApplicationStatus{Timestamp: time.Now().UTC()}
	for _, item := range payload.Items {
		app := buildApplication(item)
		if c.kube != nil {
			app.Events = c.collectEvents(ctx, app)
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
	case cmd.ArgoDelete != nil:
		path := "/api/v1/applications/" + url.PathEscape(cmd.ArgoDelete.Application)
		params := url.Values{}
		if ns := strings.TrimSpace(cmd.ArgoDelete.Namespace); ns != "" {
			params.Set("appNamespace", ns)
		}
		if cmd.ArgoDelete.Cascade {
			params.Set("cascade", "true")
		} else {
			params.Set("cascade", "false")
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
		return c.do(ctx, cmd.CommandID, http.MethodDelete, path, nil)
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

func buildApplication(item applicationPayload) domain.ArgoApplication {
	sources := normalizeSources(item.Spec.Source, item.Spec.Sources)
	sourceType := ""
	repoURL := ""
	path := ""
	targetRevision := ""
	if len(sources) > 0 {
		sourceType = sources[0].Type
		repoURL = sources[0].RepoURL
		path = sources[0].Path
		targetRevision = sources[0].TargetRevision
	}

	conditions := make([]domain.ArgoCondition, 0, len(item.Status.Conditions))
	for _, condition := range item.Status.Conditions {
		conditions = append(conditions, domain.ArgoCondition{
			Type:             condition.Type,
			Message:          condition.Message,
			LastTransitionAt: parseTimestamp(condition.LastTransitionTime),
		})
	}

	resources := make([]domain.ArgoResource, 0, len(item.Status.Resources))
	outOfSyncCount := 0
	degradedCount := 0
	for _, resource := range item.Status.Resources {
		healthStatus := ""
		healthMessage := ""
		if resource.Health != nil {
			healthStatus = resource.Health.Status
			healthMessage = resource.Health.Message
		}
		if resource.Status != "" && !strings.EqualFold(resource.Status, "Synced") {
			outOfSyncCount++
		}
		if healthStatus != "" && !strings.EqualFold(healthStatus, "Healthy") {
			degradedCount++
		}
		resources = append(resources, domain.ArgoResource{
			Group:           resource.Group,
			Kind:            resource.Kind,
			Namespace:       resource.Namespace,
			Name:            resource.Name,
			Version:         resource.Version,
			SyncStatus:      resource.Status,
			HealthStatus:    healthStatus,
			HealthMessage:   healthMessage,
			HookType:        resource.HookType,
			RequiresPruning: resource.RequiresPruning,
		})
	}

	history := make([]domain.ArgoHistoryEntry, 0, len(item.Status.History))
	var lastSyncedAt time.Time
	for _, entry := range item.Status.History {
		sourceType := ""
		if entry.Source != nil {
			sourceType = determineSourceType(*entry.Source)
		} else if len(entry.Sources) > 0 {
			sourceType = determineSourceType(entry.Sources[0])
		}
		deployedAt := parseTimestamp(entry.DeployedAt)
		if deployedAt.After(lastSyncedAt) {
			lastSyncedAt = deployedAt
		}
		history = append(history, domain.ArgoHistoryEntry{
			ID:         entry.ID,
			Revision:   entry.Revision,
			DeployedAt: deployedAt,
			SourceType: sourceType,
		})
	}

	if lastSyncedAt.IsZero() {
		lastSyncedAt = parseTimestamp(item.Status.OperationState.FinishedAt)
	}

	var syncPolicy *domain.ArgoSyncPolicy
	if item.Spec.SyncPolicy.Automated != nil {
		syncPolicy = &domain.ArgoSyncPolicy{
			Automated:  true,
			Prune:      item.Spec.SyncPolicy.Automated.Prune,
			SelfHeal:   item.Spec.SyncPolicy.Automated.SelfHeal,
			AllowEmpty: item.Spec.SyncPolicy.Automated.AllowEmpty,
		}
	}

	var operation *domain.ArgoOperationState
	if item.Status.OperationState.Phase != "" || item.Status.OperationState.Message != "" || item.Status.OperationState.StartedAt != "" || item.Status.OperationState.FinishedAt != "" {
		operation = &domain.ArgoOperationState{
			Phase:      item.Status.OperationState.Phase,
			Message:    item.Status.OperationState.Message,
			StartedAt:  parseTimestamp(item.Status.OperationState.StartedAt),
			FinishedAt: parseTimestamp(item.Status.OperationState.FinishedAt),
		}
	}

	return domain.ArgoApplication{
		Name:                   item.Metadata.Name,
		Namespace:              item.Metadata.Namespace,
		Project:                item.Spec.Project,
		RepoURL:                repoURL,
		Path:                   path,
		TargetRevision:         targetRevision,
		SyncStatus:             item.Status.Sync.Status,
		HealthStatus:           item.Status.Health.Status,
		ResourceCount:          len(resources),
		LastSyncedAt:           lastSyncedAt,
		DestinationServer:      item.Spec.Destination.Server,
		DestinationNamespace:   item.Spec.Destination.Namespace,
		SourceType:             sourceType,
		Sources:                sources,
		LiveRevision:           item.Status.Sync.Revision,
		ReconciledAt:           parseTimestamp(item.Status.ReconciledAt),
		OutOfSyncResourceCount: outOfSyncCount,
		DegradedResourceCount:  degradedCount,
		SyncPolicy:             syncPolicy,
		Operation:              operation,
		Conditions:             conditions,
		Resources:              resources,
		History:                history,
	}
}

func normalizeSources(source applicationSourcePayload, sources []applicationSourcePayload) []domain.ArgoSource {
	if len(sources) == 0 {
		sources = []applicationSourcePayload{source}
	}
	out := make([]domain.ArgoSource, 0, len(sources))
	for _, item := range sources {
		if item.RepoURL == "" && item.Path == "" && item.Chart == "" && item.TargetRevision == "" && item.Ref == "" {
			continue
		}
		out = append(out, domain.ArgoSource{
			RepoURL:        item.RepoURL,
			Path:           item.Path,
			Chart:          item.Chart,
			TargetRevision: item.TargetRevision,
			Ref:            item.Ref,
			Type:           determineSourceType(item),
		})
	}
	return out
}

func determineSourceType(source applicationSourcePayload) string {
	switch {
	case source.Chart != "":
		return "helm"
	case source.Path != "":
		return "git"
	case source.RepoURL != "":
		return "repository"
	default:
		return ""
	}
}

func parseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	formats := []string{time.RFC3339Nano, time.RFC3339}
	for _, format := range formats {
		if ts, err := time.Parse(format, raw); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

func (c *Client) collectEvents(ctx context.Context, app domain.ArgoApplication) []domain.ArgoEvent {
	if c.kube == nil {
		return nil
	}

	type subject struct {
		namespace string
		kind      string
		name      string
	}

	subjects := make(map[string]subject, maxEventSubjects)
	addSubject := func(namespace, kind, name string) {
		if namespace == "" || kind == "" || name == "" || len(subjects) >= maxEventSubjects {
			return
		}
		key := namespace + "|" + kind + "|" + name
		subjects[key] = subject{namespace: namespace, kind: kind, name: name}
	}

	addSubject(app.Namespace, "Application", app.Name)
	for _, resource := range app.Resources {
		addSubject(resource.Namespace, resource.Kind, resource.Name)
	}

	if len(subjects) == 0 {
		return nil
	}

	namespaces := make(map[string]struct{}, len(subjects))
	for _, item := range subjects {
		namespaces[item.namespace] = struct{}{}
	}

	collected := make([]domain.ArgoEvent, 0, maxApplicationEvents)
	for namespace := range namespaces {
		events, err := c.kube.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, event := range events.Items {
			key := namespace + "|" + event.InvolvedObject.Kind + "|" + event.InvolvedObject.Name
			target, ok := subjects[key]
			if !ok || event.InvolvedObject.Kind != target.kind || event.InvolvedObject.Name != target.name {
				continue
			}
			collected = append(collected, domain.ArgoEvent{
				Type:           event.Type,
				Reason:         event.Reason,
				Message:        event.Message,
				Namespace:      namespace,
				Kind:           event.InvolvedObject.Kind,
				Name:           event.InvolvedObject.Name,
				Count:          int(event.Count),
				FirstTimestamp: eventTimestamp(event, true),
				LastTimestamp:  eventTimestamp(event, false),
			})
		}
	}

	sort.SliceStable(collected, func(i, j int) bool {
		return collected[i].LastTimestamp.After(collected[j].LastTimestamp)
	})
	if len(collected) > maxApplicationEvents {
		collected = collected[:maxApplicationEvents]
	}
	return collected
}

func eventTimestamp(event corev1.Event, first bool) time.Time {
	if first {
		if !event.FirstTimestamp.IsZero() {
			return event.FirstTimestamp.UTC()
		}
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.UTC()
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time.UTC()
	}
	if !event.CreationTimestamp.IsZero() {
		return event.CreationTimestamp.UTC()
	}
	return time.Time{}
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
