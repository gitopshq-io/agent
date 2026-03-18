package port

import (
	"context"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

type HubTransport interface {
	Register(ctx context.Context, token string, cluster domain.Cluster) (domain.RegisterResponse, error)
	Connect(ctx context.Context, agentToken string) (HubSession, error)
}

type HubSession interface {
	Send(domain.AgentMessage) error
	Recv() (domain.HubMessage, error)
	CloseSend() error
}

type IdentityStore interface {
	Load(ctx context.Context) (domain.AgentIdentity, error)
	Save(ctx context.Context, identity domain.AgentIdentity) error
}

type InventoryCollector interface {
	CollectInventory(ctx context.Context) (*domain.InventorySnapshot, error)
	CollectDrift(ctx context.Context) (*domain.DriftReport, error)
}

type ApplicationObserver interface {
	CollectApplications(ctx context.Context) (*domain.ArgoApplicationStatus, error)
}

type CommandExecutor interface {
	Execute(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error)
}

type CredentialSyncer interface {
	Sync(ctx context.Context, req domain.CredentialSyncRequest) (domain.CredentialSyncResult, error)
}

type Clock interface {
	Now() time.Time
}
