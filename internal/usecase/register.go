package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type Register struct {
	Transport port.HubTransport
	Store     port.IdentityStore
}

func (u Register) Run(ctx context.Context, registrationToken string, cluster domain.Cluster) (domain.RegisterResponse, error) {
	resp, err := u.Transport.Register(ctx, registrationToken, cluster)
	if err != nil {
		return domain.RegisterResponse{}, fmt.Errorf("register agent: %w", err)
	}
	if err := u.Store.Save(ctx, domain.AgentIdentity{
		AgentToken: resp.AgentToken,
		ClusterID:  resp.ClusterID,
	}); err != nil {
		return domain.RegisterResponse{}, fmt.Errorf("persist agent identity: %w", err)
	}
	slog.Info("persisted agent identity after registration", "clusterId", resp.ClusterID)
	return resp, nil
}
