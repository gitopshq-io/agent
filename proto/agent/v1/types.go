package agentv1

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
	CapabilityArgoCDRead        Capability = "argocd.read"
	CapabilityArgoCDWrite       Capability = "argocd.write"
	CapabilityDirectDeploy      Capability = "deploy.direct"
	CapabilityKubernetesRestart Capability = "k8s.restart"
	CapabilityKubernetesScale   Capability = "k8s.scale"
	CapabilityCredentialSync    Capability = "credentials.sync"
	CapabilityTokenRotate       Capability = "token.rotate"
)

type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusAcked     CommandStatus = "acked"
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
)

type ClusterInfo struct {
	Name              string         `json:"name"`
	DisplayName       string         `json:"displayName,omitempty"`
	Provider          string         `json:"provider,omitempty"`
	Region            string         `json:"region,omitempty"`
	Environment       string         `json:"environment,omitempty"`
	AgentVersion      string         `json:"agentVersion,omitempty"`
	KubernetesVersion string         `json:"kubernetesVersion,omitempty"`
	Capabilities      []Capability   `json:"capabilities,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type RegisterRequest struct {
	RegistrationToken string      `json:"registrationToken"`
	Cluster           ClusterInfo `json:"cluster"`
}

type RegisterResponse struct {
	AgentToken            string `json:"agentToken"`
	ClusterID             string `json:"clusterId"`
	StatusIntervalSeconds int32  `json:"statusIntervalSeconds"`
}

type AgentEnvelope struct {
	Heartbeat             *Heartbeat             `json:"heartbeat,omitempty"`
	InventorySnapshot     *InventorySnapshot     `json:"inventorySnapshot,omitempty"`
	ArgoApplicationStatus *ArgoApplicationStatus `json:"argoApplicationStatus,omitempty"`
	DriftReport           *DriftReport           `json:"driftReport,omitempty"`
	CommandAck            *CommandAck            `json:"commandAck,omitempty"`
	CommandResult         *CommandResult         `json:"commandResult,omitempty"`
	CredentialSyncResult  *CredentialSyncResult  `json:"credentialSyncResult,omitempty"`
}

type HubEnvelope struct {
	ExecuteCommand  *ExecuteCommand  `json:"executeCommand,omitempty"`
	SyncCredentials *SyncCredentials `json:"syncCredentials,omitempty"`
	RotateToken     *RotateToken     `json:"rotateToken,omitempty"`
	ConfigUpdate    *ConfigUpdate    `json:"configUpdate,omitempty"`
	Ping            *Ping            `json:"ping,omitempty"`
}

type Ping struct {
	Timestamp time.Time `json:"timestamp"`
}

type Heartbeat struct {
	ClusterID    string       `json:"clusterId,omitempty"`
	AgentVersion string       `json:"agentVersion,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
	Timestamp    time.Time    `json:"timestamp"`
}

type InventorySummary struct {
	ClusterName       string `json:"clusterName,omitempty"`
	NamespaceCount    int    `json:"namespaceCount"`
	NodeCount         int    `json:"nodeCount"`
	ReadyNodeCount    int    `json:"readyNodeCount"`
	PodCount          int    `json:"podCount"`
	DeploymentCount   int    `json:"deploymentCount"`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
}

type ResourceRef struct {
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Status    string            `json:"status,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type InventorySnapshot struct {
	Timestamp time.Time        `json:"timestamp"`
	Summary   InventorySummary `json:"summary"`
	Resources []ResourceRef    `json:"resources,omitempty"`
}

type ArgoApplication struct {
	Name           string    `json:"name"`
	Namespace      string    `json:"namespace,omitempty"`
	Project        string    `json:"project,omitempty"`
	RepoURL        string    `json:"repoUrl,omitempty"`
	Path           string    `json:"path,omitempty"`
	TargetRevision string    `json:"targetRevision,omitempty"`
	SyncStatus     string    `json:"syncStatus,omitempty"`
	HealthStatus   string    `json:"healthStatus,omitempty"`
	ResourceCount  int       `json:"resourceCount"`
	LastSyncedAt   time.Time `json:"lastSyncedAt,omitempty"`
}

type ArgoApplicationStatus struct {
	Timestamp    time.Time         `json:"timestamp"`
	Applications []ArgoApplication `json:"applications,omitempty"`
}

type DriftFinding struct {
	ID             string    `json:"id,omitempty"`
	Severity       string    `json:"severity"`
	Scope          string    `json:"scope,omitempty"`
	Kind           string    `json:"kind,omitempty"`
	Namespace      string    `json:"namespace,omitempty"`
	Name           string    `json:"name,omitempty"`
	Summary        string    `json:"summary"`
	DesiredVersion string    `json:"desiredVersion,omitempty"`
	LiveVersion    string    `json:"liveVersion,omitempty"`
	DetectedAt     time.Time `json:"detectedAt"`
	Details        any       `json:"details,omitempty"`
}

type DriftReport struct {
	Timestamp time.Time      `json:"timestamp"`
	Findings  []DriftFinding `json:"findings,omitempty"`
}

type CommandAck struct {
	CommandID string        `json:"commandId"`
	Status    CommandStatus `json:"status"`
	Message   string        `json:"message,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

type CommandResult struct {
	CommandID string        `json:"commandId"`
	Status    CommandStatus `json:"status"`
	Message   string        `json:"message,omitempty"`
	Error     string        `json:"error,omitempty"`
	Result    any           `json:"result,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

type CredentialBundle struct {
	Version        string            `json:"version"`
	Namespace      string            `json:"namespace"`
	SecretName     string            `json:"secretName"`
	Type           string            `json:"type,omitempty"`
	StringData     map[string]string `json:"stringData,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	RequiredScopes []string          `json:"requiredScopes,omitempty"`
}

type CredentialSyncResult struct {
	Version    string    `json:"version"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	Namespace  string    `json:"namespace,omitempty"`
	SecretName string    `json:"secretName,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type ConfigUpdate struct {
	StatusIntervalSeconds int32        `json:"statusIntervalSeconds,omitempty"`
	Capabilities          []Capability `json:"capabilities,omitempty"`
}

type RotateToken struct {
	CommandID string    `json:"commandId"`
	NewToken  string    `json:"newToken"`
	Timestamp time.Time `json:"timestamp"`
}

type ExecuteCommand struct {
	CommandID           string                      `json:"commandId,omitempty"`
	RequiredCapability  Capability                  `json:"requiredCapability,omitempty"`
	ExpiresAt           time.Time                   `json:"expiresAt,omitempty"`
	SpecHash            string                      `json:"specHash,omitempty"`
	RequestedBy         string                      `json:"requestedBy,omitempty"`
	ArgoSync            *ArgoSyncCommand            `json:"argoSync,omitempty"`
	ArgoRollback        *ArgoRollbackCommand        `json:"argoRollback,omitempty"`
	DeployHelmRelease   *DeployHelmReleaseCommand   `json:"deployHelmRelease,omitempty"`
	ApplyKustomize      *ApplyKustomizeCommand      `json:"applyKustomize,omitempty"`
	ApplyManifestBundle *ApplyManifestBundleCommand `json:"applyManifestBundle,omitempty"`
	RestartWorkload     *RestartWorkloadCommand     `json:"restartWorkload,omitempty"`
	ScaleWorkload       *ScaleWorkloadCommand       `json:"scaleWorkload,omitempty"`
	RunDriftScan        *RunDriftScanCommand        `json:"runDriftScan,omitempty"`
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

type SyncCredentials struct {
	CommandID string             `json:"commandId"`
	Version   string             `json:"version"`
	Bundles   []CredentialBundle `json:"bundles"`
}

func (c *ExecuteCommand) Kind() string {
	switch {
	case c == nil:
		return ""
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
	default:
		return ""
	}
}

func (c *ExecuteCommand) InferRequiredCapability() Capability {
	if c == nil {
		return CapabilityObserve
	}
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
	default:
		return CapabilityObserve
	}
}

func (c *ExecuteCommand) Validate() error {
	if c == nil {
		return errors.New("command is nil")
	}
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

type executeCommandSpec struct {
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
}

func (c *ExecuteCommand) spec() (executeCommandSpec, error) {
	if err := c.Validate(); err != nil {
		return executeCommandSpec{}, err
	}
	requiredCapability := c.RequiredCapability
	if requiredCapability == "" {
		requiredCapability = expectedRequiredCapability(c)
	}
	if requiredCapability == "" {
		return executeCommandSpec{}, errors.New("required capability is missing")
	}
	if c.ExpiresAt.IsZero() {
		return executeCommandSpec{}, errors.New("expiresAt is required")
	}
	return executeCommandSpec{
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
	}, nil
}

func (c *ExecuteCommand) SpecDigest() (string, error) {
	spec, err := c.spec()
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
	spec, err := c.spec()
	if err != nil {
		return err
	}
	sum, err := c.SpecDigest()
	if err != nil {
		return err
	}
	c.RequiredCapability = spec.RequiredCapability
	c.ExpiresAt = spec.ExpiresAt
	c.SpecHash = sum
	return nil
}

func (c *ExecuteCommand) Verify(now time.Time) error {
	spec, err := c.spec()
	if err != nil {
		return err
	}
	if c.SpecHash == "" {
		return errors.New("spec hash is required")
	}
	if !spec.ExpiresAt.After(now.UTC()) {
		return fmt.Errorf("command expired at %s", spec.ExpiresAt.Format(time.RFC3339))
	}
	if spec.RequiredCapability != expectedRequiredCapability(c) {
		return fmt.Errorf("required capability %q does not match command kind %q", spec.RequiredCapability, c.Kind())
	}
	sum, err := c.SpecDigest()
	if err != nil {
		return err
	}
	if sum != c.SpecHash {
		return errors.New("spec hash mismatch")
	}
	return nil
}

func expectedRequiredCapability(c *ExecuteCommand) Capability {
	if c == nil {
		return CapabilityObserve
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
	default:
		return CapabilityObserve
	}
}
