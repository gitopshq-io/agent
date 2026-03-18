package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gitopshq-io/agent/internal/adapter/argocd"
	"github.com/gitopshq-io/agent/internal/adapter/credentialsync"
	"github.com/gitopshq-io/agent/internal/adapter/directdeploy"
	"github.com/gitopshq-io/agent/internal/adapter/hubgrpc"
	"github.com/gitopshq-io/agent/internal/adapter/kubernetes"
	runtimeadapter "github.com/gitopshq-io/agent/internal/adapter/runtime"
	sourceadapter "github.com/gitopshq-io/agent/internal/adapter/source"
	"github.com/gitopshq-io/agent/internal/domain"
	cfgpkg "github.com/gitopshq-io/agent/internal/platform/config"
	"github.com/gitopshq-io/agent/internal/port"
	"github.com/gitopshq-io/agent/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := cfgpkg.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid agent configuration", "error", err)
		os.Exit(1)
	}

	cluster := domain.Cluster{
		Name:         cfg.Agent.ClusterName,
		DisplayName:  cfg.Agent.DisplayName,
		Provider:     cfg.Agent.Provider,
		Region:       cfg.Agent.Region,
		Environment:  cfg.Agent.Environment,
		AgentVersion: cfg.Agent.Version,
		Capabilities: parseCapabilities(cfg.Agent.Capabilities),
	}

	hub := hubgrpc.New(cfg.Hub)
	normalizedArgoServer := argocd.NormalizeServerURL(cfg.ArgoCD.ServerURL, cfg.ArgoCD.Insecure)
	switch {
	case normalizedArgoServer == "":
		slog.Info("argocd integration disabled", "reason", "GITOPSHQ_ARGOCD_SERVER is empty", "capabilityEnabled", hasCapability(cluster.Capabilities, domain.CapabilityArgoCDRead) || hasCapability(cluster.Capabilities, domain.CapabilityArgoCDWrite))
	case cfg.ArgoCD.Token == "":
		slog.Warn("argocd integration is enabled without a token", "server", normalizedArgoServer, "rawServer", cfg.ArgoCD.ServerURL, "insecure", cfg.ArgoCD.Insecure)
	default:
		slog.Info("argocd integration enabled", "server", normalizedArgoServer, "rawServer", cfg.ArgoCD.ServerURL, "insecure", cfg.ArgoCD.Insecure)
	}
	kubeClient, err := kubernetes.New(cfg.DirectDeploy)
	if err != nil {
		slog.Error("failed to initialize kubernetes runtime", "error", err)
		os.Exit(1)
	}
	argocdClient := argocd.New(cfg.ArgoCD, kubeClient.TypedClient())
	identityStore, identityLocation, err := buildIdentityStore(cfg, kubeClient)
	if err != nil {
		slog.Error("failed to initialize identity store", "error", err)
		os.Exit(1)
	}
	slog.Info("agent identity store configured", "mode", cfg.Identity.Mode, "location", identityLocation)
	register := usecase.Register{Transport: hub, Store: identityStore}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	identity, err := identityStore.Load(ctx)
	statusInterval := cfg.Hub.StatusInterval
	if err != nil {
		if cfg.Hub.RegistrationToken == "" {
			slog.Error("agent identity is missing and no registration token was provided", "store", cfg.Identity.Mode, "location", identityLocation, "error", err)
			os.Exit(1)
		}
		slog.Warn("persisted agent identity was not available; attempting bootstrap registration", "store", cfg.Identity.Mode, "location", identityLocation, "error", err)
		resp, regErr := register.Run(ctx, cfg.Hub.RegistrationToken, cluster)
		if regErr != nil {
			slog.Error("agent registration failed", "error", regErr)
			os.Exit(1)
		}
		identity = domain.AgentIdentity{
			AgentToken: resp.AgentToken,
			ClusterID:  resp.ClusterID,
		}
		if resp.StatusInterval > 0 {
			statusInterval = resp.StatusInterval
		}
		slog.Info("agent registered", "clusterId", resp.ClusterID, "identityStore", cfg.Identity.Mode, "identityLocation", identityLocation)
	} else {
		slog.Info("using persisted agent identity", "clusterId", identity.ClusterID, "identityStore", cfg.Identity.Mode, "identityLocation", identityLocation)
	}
	sourceLoader := sourceadapter.Loader{
		WorkDir: cfg.DirectDeploy.WorkDir,
		Secrets: kubeClient,
	}
	directExecutor := directdeploy.Executor{
		Runtime: kubeClient,
		Sources: sourceLoader,
	}
	clock := runtimeadapter.RealClock{}
	state := usecase.NewSessionState(identity.ClusterID, cfg.Agent.Version, statusInterval, cluster.Capabilities)
	reporter := usecase.SessionReporter{
		Inventory:    usecase.CollectInventory{Collector: kubeClient, Clock: clock},
		Drift:        usecase.ReportDrift{Collector: kubeClient, Clock: clock},
		Applications: argocdClient,
		Clock:        clock,
	}
	handler := usecase.HubMessageHandler{
		Store:          identityStore,
		Executor:       usecase.ExecuteCommand{Executor: runtimeadapter.CommandRouter{Argo: argocdClient, Direct: directExecutor}, Clock: clock},
		CredentialSync: usecase.SyncCredentials{Syncer: credentialsync.Syncer{Kubernetes: kubeClient, Targets: cfg.CredentialSync.Targets}, Clock: clock},
		State:          state,
		Clock:          clock,
	}
	session := usecase.MaintainSession{
		Transport: hub,
		Store:     identityStore,
		Reporter:  reporter,
		Handler:   handler,
		State:     state,
	}

	backoff := time.Second
	for {
		err := session.Run(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}
		if cfg.Hub.RegistrationToken != "" && shouldBootstrapWithRegistrationToken(err) {
			slog.Warn("persisted agent identity was rejected by hub; attempting re-registration", "error", err)
			resp, regErr := register.Run(ctx, cfg.Hub.RegistrationToken, cluster)
			if regErr != nil {
				slog.Error("agent re-registration failed", "error", regErr)
			} else {
				if resp.StatusInterval > 0 {
					statusInterval = resp.StatusInterval
					state.ApplyConfigUpdate(domain.ConfigUpdate{StatusInterval: statusInterval})
				}
				slog.Info("agent re-registered after identity rejection", "clusterId", resp.ClusterID, "identityStore", cfg.Identity.Mode, "identityLocation", identityLocation)
				backoff = time.Second
				continue
			}
		}
		slog.Warn("agent session ended; reconnecting", "error", err, "backoff", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func shouldBootstrapWithRegistrationToken(err error) bool {
	if err == nil {
		return false
	}
	return status.Code(err) == codes.Unauthenticated
}

func parseCapabilities(values []string) []domain.Capability {
	out := make([]domain.Capability, 0, len(values))
	for _, value := range values {
		out = append(out, domain.Capability(value))
	}
	return out
}

func hasCapability(values []domain.Capability, want domain.Capability) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func buildIdentityStore(cfg cfgpkg.Config, kubeClient *kubernetes.Client) (port.IdentityStore, string, error) {
	switch cfg.Identity.Mode {
	case "file":
		return runtimeadapter.FileIdentityStore{Path: cfg.Identity.FilePath}, cfg.Identity.FilePath, nil
	case "secret":
		namespace := strings.TrimSpace(cfg.Identity.SecretNamespace)
		if namespace == "" {
			return nil, "", fmt.Errorf("identity secret namespace is required for secret identity store")
		}
		return runtimeadapter.SecretIdentityStore{
			Client:     kubeClient.TypedClient(),
			Namespace:  namespace,
			SecretName: cfg.Identity.SecretName,
		}, namespace + "/" + cfg.Identity.SecretName, nil
	default:
		return nil, "", fmt.Errorf("unsupported identity store mode %q", cfg.Identity.Mode)
	}
}
