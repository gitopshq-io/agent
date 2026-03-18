package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
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
	"github.com/gitopshq-io/agent/internal/usecase"
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
	identityStore := runtimeadapter.FileIdentityStore{Path: cfg.Hub.AgentTokenPath}
	register := usecase.Register{Transport: hub, Store: identityStore}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	identity, err := identityStore.Load(ctx)
	statusInterval := cfg.Hub.StatusInterval
	if err != nil {
		if cfg.Hub.RegistrationToken == "" {
			slog.Error("agent identity is missing and no registration token was provided", "path", cfg.Hub.AgentTokenPath)
			os.Exit(1)
		}
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
		slog.Info("agent registered", "clusterId", resp.ClusterID)
	}

	argocdClient := argocd.New(cfg.ArgoCD)
	kubeClient, err := kubernetes.New(cfg.DirectDeploy)
	if err != nil {
		slog.Error("failed to initialize kubernetes runtime", "error", err)
		os.Exit(1)
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

func parseCapabilities(values []string) []domain.Capability {
	out := make([]domain.Capability, 0, len(values))
	for _, value := range values {
		out = append(out, domain.Capability(value))
	}
	return out
}
