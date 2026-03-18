package config

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Hub            HubConfig
	Identity       IdentityConfig
	Agent          AgentConfig
	Diagnostics    DiagnosticsConfig
	ArgoCD         ArgoCDConfig
	CredentialSync CredentialSyncConfig
	DirectDeploy   DirectDeployConfig
}

type HubConfig struct {
	Address           string
	Insecure          bool
	StatusInterval    time.Duration
	RegistrationToken string
}

type IdentityConfig struct {
	Mode            string
	FilePath        string
	SecretName      string
	SecretNamespace string
}

type AgentConfig struct {
	ClusterName  string
	DisplayName  string
	Provider     string
	Region       string
	Environment  string
	Version      string
	Capabilities []string
}

type DiagnosticsConfig struct {
	AllowedNamespaces []string
}

type ArgoCDConfig struct {
	ServerURL string
	Token     string
	Insecure  bool
}

type CredentialSyncConfig struct {
	Mode    string
	Targets []string
}

type DirectDeployConfig struct {
	KubeconfigPath   string
	DefaultNamespace string
	FieldManager     string
	ForceOwnership   bool
	WorkDir          string
}

func Load() Config {
	return Config{
		Hub: HubConfig{
			Address:           envOrDefault("GITOPSHQ_HUB_ADDRESS", "127.0.0.1:50051"),
			Insecure:          envBoolOrDefault("GITOPSHQ_HUB_INSECURE", false),
			StatusInterval:    time.Duration(envIntOrDefault("GITOPSHQ_STATUS_INTERVAL_SECONDS", 30)) * time.Second,
			RegistrationToken: envOrDefault("GITOPSHQ_REGISTRATION_TOKEN", ""),
		},
		Identity: IdentityConfig{
			Mode:            strings.ToLower(envOrDefault("GITOPSHQ_IDENTITY_STORE_MODE", "file")),
			FilePath:        envOrDefault("GITOPSHQ_AGENT_TOKEN_PATH", "/tmp/gitopshq-agent-token"),
			SecretName:      envOrDefault("GITOPSHQ_IDENTITY_SECRET_NAME", ""),
			SecretNamespace: envOrDefault("GITOPSHQ_IDENTITY_SECRET_NAMESPACE", ""),
		},
		Agent: AgentConfig{
			ClusterName:  envOrDefault("GITOPSHQ_CLUSTER_NAME", "default"),
			DisplayName:  envOrDefault("GITOPSHQ_CLUSTER_DISPLAY_NAME", ""),
			Provider:     envOrDefault("GITOPSHQ_CLUSTER_PROVIDER", ""),
			Region:       envOrDefault("GITOPSHQ_CLUSTER_REGION", ""),
			Environment:  envOrDefault("GITOPSHQ_CLUSTER_ENVIRONMENT", ""),
			Version:      envOrDefault("GITOPSHQ_AGENT_VERSION", "dev"),
			Capabilities: splitCSV(envOrDefault("GITOPSHQ_CAPABILITIES", "observe")),
		},
		Diagnostics: DiagnosticsConfig{
			AllowedNamespaces: splitCSV(envOrDefault("GITOPSHQ_DIAGNOSTICS_ALLOWED_NAMESPACES", "")),
		},
		ArgoCD: ArgoCDConfig{
			ServerURL: envOrDefault("GITOPSHQ_ARGOCD_SERVER", ""),
			Token:     envOrDefault("GITOPSHQ_ARGOCD_TOKEN", ""),
			Insecure:  envBoolOrDefault("GITOPSHQ_ARGOCD_INSECURE", false),
		},
		CredentialSync: CredentialSyncConfig{
			Mode:    envOrDefault("GITOPSHQ_CREDENTIAL_SYNC_MODE", "mirrored"),
			Targets: splitCSV(envOrDefault("GITOPSHQ_CREDENTIAL_SYNC_TARGETS", "")),
		},
		DirectDeploy: DirectDeployConfig{
			KubeconfigPath:   envOrDefault("GITOPSHQ_KUBECONFIG", ""),
			DefaultNamespace: envOrDefault("GITOPSHQ_DIRECT_DEPLOY_DEFAULT_NAMESPACE", ""),
			FieldManager:     envOrDefault("GITOPSHQ_DIRECT_DEPLOY_FIELD_MANAGER", "gitopshq-agent"),
			ForceOwnership:   envBoolOrDefault("GITOPSHQ_DIRECT_DEPLOY_FORCE_OWNERSHIP", false),
			WorkDir:          envOrDefault("GITOPSHQ_DIRECT_DEPLOY_WORKDIR", os.TempDir()),
		},
	}
}

func (c Config) Validate() error {
	switch {
	case c.Hub.Address == "":
		return fmt.Errorf("hub address is required")
	case c.Agent.ClusterName == "":
		return fmt.Errorf("cluster name is required")
	case c.Hub.StatusInterval <= 0:
		return fmt.Errorf("status interval must be greater than zero")
	case c.DirectDeploy.FieldManager == "":
		return fmt.Errorf("direct deploy field manager is required")
	case c.DirectDeploy.WorkDir == "":
		return fmt.Errorf("direct deploy workdir is required")
	case c.Identity.Mode != "file" && c.Identity.Mode != "secret":
		return fmt.Errorf("identity store mode must be one of: file, secret")
	case c.Identity.Mode == "file" && c.Identity.FilePath == "":
		return fmt.Errorf("agent token path is required when using file identity store")
	case c.Identity.Mode == "secret" && c.Identity.SecretName == "":
		return fmt.Errorf("identity secret name is required when using secret identity store")
	case c.Identity.Mode == "secret" && c.Identity.SecretNamespace == "":
		return fmt.Errorf("identity secret namespace is required when using secret identity store")
	default:
		return nil
	}
}

func (c HubConfig) TLSConfig() (*tls.Config, error) {
	if c.Insecure {
		return nil, nil
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			if i > start {
				trimmed := strings.TrimSpace(v[start:i])
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
			start = i + 1
		}
	}
	return out
}
