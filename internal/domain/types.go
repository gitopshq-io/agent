package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Capability string

const (
	CapabilityObserve           Capability = "observe"
	CapabilityDiagnosticsRead   Capability = "diagnostics.read"
	CapabilityArgoCDRead        Capability = "argocd.read"
	CapabilityArgoCDWrite       Capability = "argocd.write"
	CapabilityDirectDeploy      Capability = "deploy.direct"
	CapabilityKubernetesRestart Capability = "k8s.restart"
	CapabilityKubernetesScale   Capability = "k8s.scale"
	CapabilityCredentialSync    Capability = "credentials.sync"
	CapabilityTokenRotate       Capability = "token.rotate"
)

type CapabilitySet map[Capability]struct{}

func NewCapabilitySet(capabilities []Capability) CapabilitySet {
	set := make(CapabilitySet, len(capabilities))
	for _, capability := range capabilities {
		if capability == "" {
			continue
		}
		set[capability] = struct{}{}
	}
	return set
}

func (s CapabilitySet) Has(capability Capability) bool {
	if capability == "" {
		return false
	}
	_, ok := s[capability]
	return ok
}

func (s CapabilitySet) Values() []Capability {
	out := make([]Capability, 0, len(s))
	for capability := range s {
		out = append(out, capability)
	}
	return out
}

type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusAcked     CommandStatus = "acked"
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
)

type Cluster struct {
	ID                string
	Name              string
	DisplayName       string
	Provider          string
	Region            string
	Environment       string
	AgentVersion      string
	KubernetesVersion string
	Capabilities      []Capability
	Metadata          map[string]any
}

type AgentIdentity struct {
	AgentToken string
	ClusterID  string
}

type RegisterResponse struct {
	AgentToken     string
	ClusterID      string
	StatusInterval time.Duration
}

type AgentMessage struct {
	Heartbeat            *Heartbeat
	InventorySnapshot    *InventorySnapshot
	ApplicationStatus    *ArgoApplicationStatus
	DriftReport          *DriftReport
	CommandAck           *CommandAck
	CommandResult        *CommandResult
	CredentialSyncResult *CredentialSyncResult
}

type HubMessage struct {
	ExecuteCommand  *ExecuteCommand
	SyncCredentials *CredentialSyncRequest
	RotateToken     *RotateToken
	ConfigUpdate    *ConfigUpdate
	Ping            *Ping
}

type Ping struct {
	Timestamp time.Time
}

type Heartbeat struct {
	ClusterID    string
	AgentVersion string
	Capabilities []Capability
	Timestamp    time.Time
}

type InventorySummary struct {
	ClusterName       string
	NamespaceCount    int
	NodeCount         int
	ReadyNodeCount    int
	PodCount          int
	DeploymentCount   int
	KubernetesVersion string
}

type ResourceRef struct {
	Kind      string
	Namespace string
	Name      string
	Status    string
	Labels    map[string]string
}

type InventorySnapshot struct {
	Timestamp time.Time
	Summary   InventorySummary
	Resources []ResourceRef
}

type ArgoSource struct {
	RepoURL        string
	Path           string
	Chart          string
	TargetRevision string
	Ref            string
	Type           string
}

type ArgoSyncPolicy struct {
	Automated  bool
	Prune      bool
	SelfHeal   bool
	AllowEmpty bool
}

type ArgoOperationState struct {
	Phase      string
	Message    string
	StartedAt  time.Time
	FinishedAt time.Time
}

type ArgoCondition struct {
	Type             string
	Message          string
	LastTransitionAt time.Time
}

type ArgoResource struct {
	Group           string
	Kind            string
	Namespace       string
	Name            string
	Version         string
	SyncStatus      string
	HealthStatus    string
	HealthMessage   string
	HookType        string
	RequiresPruning bool
}

type ArgoEvent struct {
	Type           string
	Reason         string
	Message        string
	Namespace      string
	Kind           string
	Name           string
	Count          int
	FirstTimestamp time.Time
	LastTimestamp  time.Time
}

type ArgoHistoryEntry struct {
	ID         int64
	Revision   string
	DeployedAt time.Time
	SourceType string
}

type ArgoApplication struct {
	Name                   string
	Namespace              string
	Project                string
	RepoURL                string
	Path                   string
	TargetRevision         string
	SyncStatus             string
	HealthStatus           string
	ResourceCount          int
	LastSyncedAt           time.Time
	DestinationServer      string
	DestinationNamespace   string
	SourceType             string
	Sources                []ArgoSource
	LiveRevision           string
	ReconciledAt           time.Time
	OutOfSyncResourceCount int
	DegradedResourceCount  int
	SyncPolicy             *ArgoSyncPolicy
	Operation              *ArgoOperationState
	Conditions             []ArgoCondition
	Resources              []ArgoResource
	Events                 []ArgoEvent
	History                []ArgoHistoryEntry
}

type ArgoApplicationStatus struct {
	Timestamp    time.Time
	Applications []ArgoApplication
}

type DriftFinding struct {
	ID             string
	Severity       string
	Scope          string
	Kind           string
	Namespace      string
	Name           string
	Summary        string
	DesiredVersion string
	LiveVersion    string
	DetectedAt     time.Time
	Details        any
}

type DriftReport struct {
	Timestamp time.Time
	Findings  []DriftFinding
}

type CommandAck struct {
	CommandID string
	Status    CommandStatus
	Message   string
	Timestamp time.Time
}

type CommandResult struct {
	CommandID string
	Status    CommandStatus
	Message   string
	Error     string
	Result    any
	Timestamp time.Time
}

type CredentialBundle struct {
	Version        string
	Namespace      string
	SecretName     string
	Type           string
	StringData     map[string]string
	Labels         map[string]string
	Annotations    map[string]string
	RequiredScopes []string
}

type CredentialSyncRequest struct {
	CommandID string
	Version   string
	Bundles   []CredentialBundle
}

type CredentialSyncResult struct {
	Version    string
	Status     string
	Message    string
	Namespace  string
	SecretName string
	Timestamp  time.Time
}

type ConfigUpdate struct {
	StatusInterval time.Duration
	Capabilities   []Capability
}

type RotateToken struct {
	CommandID string
	NewToken  string
	Timestamp time.Time
}

type ExecutionSpec struct {
	RequiredCapability  Capability                  `json:"requiredCapability"`
	ExpiresAt           time.Time                   `json:"expiresAt"`
	ArgoSync            *ArgoSyncCommand            `json:"argoSync,omitempty"`
	ArgoRollback        *ArgoRollbackCommand        `json:"argoRollback,omitempty"`
	DeployHelmRelease   *DeployHelmReleaseCommand   `json:"deployHelmRelease,omitempty"`
	ApplyKustomize      *ApplyKustomizeCommand      `json:"applyKustomize,omitempty"`
	ApplyManifestBundle *ApplyManifestBundleCommand `json:"applyManifestBundle,omitempty"`
	RestartWorkload     *RestartWorkloadCommand     `json:"restartWorkload,omitempty"`
	ScaleWorkload       *ScaleWorkloadCommand       `json:"scaleWorkload,omitempty"`
	RunDriftScan        *RunDriftScanCommand        `json:"runDriftScan,omitempty"`
	InspectResource     *InspectResourceCommand     `json:"inspectResource,omitempty"`
}

type ExecuteCommand struct {
	CommandID           string
	RequiredCapability  Capability
	ExpiresAt           time.Time
	SpecHash            string
	RequestedBy         string
	ArgoSync            *ArgoSyncCommand
	ArgoRollback        *ArgoRollbackCommand
	DeployHelmRelease   *DeployHelmReleaseCommand
	ApplyKustomize      *ApplyKustomizeCommand
	ApplyManifestBundle *ApplyManifestBundleCommand
	RestartWorkload     *RestartWorkloadCommand
	ScaleWorkload       *ScaleWorkloadCommand
	RunDriftScan        *RunDriftScanCommand
	InspectResource     *InspectResourceCommand
}

type CredentialRef struct {
	Namespace  string `json:"namespace"`
	SecretName string `json:"secretName"`
	Key        string `json:"key,omitempty"`
}

type SourceRef struct {
	Type             string         `json:"type"`
	URL              string         `json:"url"`
	ResolvedRevision string         `json:"resolvedRevision,omitempty"`
	ResolvedDigest   string         `json:"resolvedDigest,omitempty"`
	Chart            string         `json:"chart,omitempty"`
	Path             string         `json:"path,omitempty"`
	CredentialRef    *CredentialRef `json:"credentialRef,omitempty"`
}

type ValuesRef struct {
	Digest        string         `json:"digest"`
	InlineValues  map[string]any `json:"inlineValues,omitempty"`
	CredentialRef *CredentialRef `json:"credentialRef,omitempty"`
}

type ArgoSyncCommand struct {
	Application string `json:"application"`
	Namespace   string `json:"namespace,omitempty"`
	Project     string `json:"project,omitempty"`
	Prune       bool   `json:"prune,omitempty"`
	DryRun      bool   `json:"dryRun,omitempty"`
}

type ArgoRollbackCommand struct {
	Application string `json:"application"`
	Namespace   string `json:"namespace,omitempty"`
	ID          int64  `json:"id"`
	Prune       bool   `json:"prune,omitempty"`
}

type DeployHelmReleaseCommand struct {
	ReleaseName string    `json:"releaseName"`
	Namespace   string    `json:"namespace"`
	Source      SourceRef `json:"source"`
	Values      ValuesRef `json:"values"`
}

type ApplyKustomizeCommand struct {
	Namespace string     `json:"namespace"`
	Source    SourceRef  `json:"source"`
	Values    *ValuesRef `json:"values,omitempty"`
}

type ApplyManifestBundleCommand struct {
	Namespace string    `json:"namespace"`
	Source    SourceRef `json:"source"`
}

type RestartWorkloadCommand struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

type ScaleWorkloadCommand struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
}

type RunDriftScanCommand struct {
	Scope string `json:"scope,omitempty"`
}

type InspectResourceCommand struct {
	Namespace     string `json:"namespace"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Container     string `json:"container,omitempty"`
	TailLines     int32  `json:"tailLines,omitempty"`
	IncludeEvents bool   `json:"includeEvents,omitempty"`
	IncludeLogs   bool   `json:"includeLogs,omitempty"`
}

type ResourceInspection struct {
	Namespace     string           `json:"namespace"`
	Kind          string           `json:"kind"`
	Name          string           `json:"name"`
	TotalPods     int              `json:"totalPods"`
	TruncatedPods bool             `json:"truncatedPods,omitempty"`
	Pods          []InspectedPod   `json:"pods,omitempty"`
	Events        []InspectedEvent `json:"events,omitempty"`
	Logs          []InspectedLog   `json:"logs,omitempty"`
	GeneratedAt   time.Time        `json:"generatedAt,omitempty"`
}

type InspectedPod struct {
	Name            string    `json:"name"`
	Namespace       string    `json:"namespace"`
	Phase           string    `json:"phase,omitempty"`
	NodeName        string    `json:"nodeName,omitempty"`
	Containers      []string  `json:"containers,omitempty"`
	ReadyContainers int32     `json:"readyContainers,omitempty"`
	TotalContainers int32     `json:"totalContainers,omitempty"`
	Restarts        int32     `json:"restarts,omitempty"`
	StartTime       time.Time `json:"startTime,omitempty"`
}

type InspectedEvent struct {
	Type           string    `json:"type,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	Message        string    `json:"message,omitempty"`
	Namespace      string    `json:"namespace,omitempty"`
	Kind           string    `json:"kind,omitempty"`
	Name           string    `json:"name,omitempty"`
	Count          int       `json:"count,omitempty"`
	FirstTimestamp time.Time `json:"firstTimestamp,omitempty"`
	LastTimestamp  time.Time `json:"lastTimestamp,omitempty"`
}

type InspectedLog struct {
	PodName     string    `json:"podName"`
	Namespace   string    `json:"namespace"`
	Container   string    `json:"container,omitempty"`
	Content     string    `json:"content,omitempty"`
	Truncated   bool      `json:"truncated,omitempty"`
	CollectedAt time.Time `json:"collectedAt,omitempty"`
}

func (c ExecuteCommand) Kind() string {
	switch {
	case c.ArgoSync != nil:
		return "argo_sync"
	case c.ArgoRollback != nil:
		return "argo_rollback"
	case c.DeployHelmRelease != nil:
		return "deploy_helm_release"
	case c.ApplyKustomize != nil:
		return "apply_kustomize"
	case c.ApplyManifestBundle != nil:
		return "apply_manifest_bundle"
	case c.RestartWorkload != nil:
		return "restart_workload"
	case c.ScaleWorkload != nil:
		return "scale_workload"
	case c.RunDriftScan != nil:
		return "run_drift_scan"
	case c.InspectResource != nil:
		return "inspect_resource"
	default:
		return ""
	}
}

func (c ExecuteCommand) InferRequiredCapability() Capability {
	if c.RequiredCapability != "" {
		return c.RequiredCapability
	}
	switch c.Kind() {
	case "argo_sync", "argo_rollback":
		return CapabilityArgoCDWrite
	case "deploy_helm_release", "apply_kustomize", "apply_manifest_bundle":
		return CapabilityDirectDeploy
	case "restart_workload":
		return CapabilityKubernetesRestart
	case "scale_workload":
		return CapabilityKubernetesScale
	case "run_drift_scan":
		return CapabilityObserve
	case "inspect_resource":
		return CapabilityDiagnosticsRead
	default:
		return CapabilityObserve
	}
}

func (c ExecuteCommand) Validate() error {
	if c.CommandID == "" {
		return errors.New("command id is required")
	}
	count := 0
	for _, present := range []bool{
		c.ArgoSync != nil,
		c.ArgoRollback != nil,
		c.DeployHelmRelease != nil,
		c.ApplyKustomize != nil,
		c.ApplyManifestBundle != nil,
		c.RestartWorkload != nil,
		c.ScaleWorkload != nil,
		c.RunDriftScan != nil,
		c.InspectResource != nil,
	} {
		if present {
			count++
		}
	}
	if count != 1 {
		return errors.New("exactly one command payload must be set")
	}
	return nil
}

func (c ExecuteCommand) ExecutionSpec() (ExecutionSpec, error) {
	if err := c.Validate(); err != nil {
		return ExecutionSpec{}, err
	}
	requiredCapability := c.RequiredCapability
	if requiredCapability == "" {
		requiredCapability = c.InferRequiredCapability()
	}
	if requiredCapability == "" {
		return ExecutionSpec{}, errors.New("required capability is missing")
	}
	if c.ExpiresAt.IsZero() {
		return ExecutionSpec{}, errors.New("expiresAt is required")
	}
	return ExecutionSpec{
		RequiredCapability:  requiredCapability,
		ExpiresAt:           c.ExpiresAt.UTC(),
		ArgoSync:            c.ArgoSync,
		ArgoRollback:        c.ArgoRollback,
		DeployHelmRelease:   c.DeployHelmRelease,
		ApplyKustomize:      c.ApplyKustomize,
		ApplyManifestBundle: c.ApplyManifestBundle,
		RestartWorkload:     c.RestartWorkload,
		ScaleWorkload:       c.ScaleWorkload,
		RunDriftScan:        c.RunDriftScan,
		InspectResource:     c.InspectResource,
	}, nil
}

func (c ExecuteCommand) SpecDigest() (string, error) {
	spec, err := c.ExecutionSpec()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (c *ExecuteCommand) EnsureSpecHash() error {
	if c == nil {
		return errors.New("command is nil")
	}
	spec, err := c.ExecutionSpec()
	if err != nil {
		return err
	}
	c.RequiredCapability = spec.RequiredCapability
	c.ExpiresAt = spec.ExpiresAt
	sum, err := c.SpecDigest()
	if err != nil {
		return err
	}
	c.SpecHash = sum
	return nil
}

func (c ExecuteCommand) Verify(now time.Time) error {
	spec, err := c.ExecutionSpec()
	if err != nil {
		return err
	}
	if c.SpecHash == "" {
		return errors.New("spec hash is required")
	}
	if !spec.ExpiresAt.After(now.UTC()) {
		return fmt.Errorf("command expired at %s", spec.ExpiresAt.Format(time.RFC3339))
	}
	inferred := ExecuteCommand{
		ArgoSync:            c.ArgoSync,
		ArgoRollback:        c.ArgoRollback,
		DeployHelmRelease:   c.DeployHelmRelease,
		ApplyKustomize:      c.ApplyKustomize,
		ApplyManifestBundle: c.ApplyManifestBundle,
		RestartWorkload:     c.RestartWorkload,
		ScaleWorkload:       c.ScaleWorkload,
		RunDriftScan:        c.RunDriftScan,
		InspectResource:     c.InspectResource,
	}.InferRequiredCapability()
	if spec.RequiredCapability != inferred {
		return fmt.Errorf("required capability %q does not match command kind %q", spec.RequiredCapability, c.Kind())
	}
	digest, err := c.SpecDigest()
	if err != nil {
		return err
	}
	if digest != c.SpecHash {
		return errors.New("spec hash mismatch")
	}
	return nil
}
