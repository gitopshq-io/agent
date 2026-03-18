package usecase

import (
	"context"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type HubMessageHandler struct {
	Store          port.IdentityStore
	Executor       ExecuteCommand
	CredentialSync SyncCredentials
	State          *SessionState
	Clock          port.Clock
}

func (h HubMessageHandler) Handle(ctx context.Context, msg domain.HubMessage) ([]domain.AgentMessage, error) {
	switch {
	case msg.ExecuteCommand != nil:
		ack := domain.AgentMessage{
			CommandAck: &domain.CommandAck{
				CommandID: msg.ExecuteCommand.CommandID,
				Status:    domain.CommandStatusAcked,
				Timestamp: now(h.Clock),
			},
		}
		result, err := h.Executor.Run(ctx, *msg.ExecuteCommand, h.State.Capabilities())
		if err != nil {
			return nil, err
		}
		return []domain.AgentMessage{
			ack,
			{CommandResult: &result},
		}, nil
	case msg.SyncCredentials != nil:
		if !h.State.Capabilities().Has(domain.CapabilityCredentialSync) {
			result := domain.CredentialSyncResult{
				Version:   msg.SyncCredentials.Version,
				Status:    "failed",
				Message:   "credentials.sync capability is not enabled",
				Timestamp: now(h.Clock),
			}
			return []domain.AgentMessage{{CredentialSyncResult: &result}}, nil
		}
		result, err := h.CredentialSync.Run(ctx, *msg.SyncCredentials)
		if err != nil {
			failed := domain.CredentialSyncResult{
				Version:   msg.SyncCredentials.Version,
				Status:    "failed",
				Message:   err.Error(),
				Timestamp: now(h.Clock),
			}
			return []domain.AgentMessage{{CredentialSyncResult: &failed}}, nil
		}
		return []domain.AgentMessage{{CredentialSyncResult: &result}}, nil
	case msg.RotateToken != nil:
		if !h.State.Capabilities().Has(domain.CapabilityTokenRotate) {
			result := failedCommandResult(msg.RotateToken.CommandID, "token.rotate capability is not enabled", h.Clock)
			return []domain.AgentMessage{{CommandResult: &result}}, nil
		}
		identity, err := h.Store.Load(ctx)
		if err != nil {
			return nil, err
		}
		identity.AgentToken = msg.RotateToken.NewToken
		if err := h.Store.Save(ctx, identity); err != nil {
			return nil, err
		}
		if msg.RotateToken.CommandID == "" {
			return nil, nil
		}
		result := domain.CommandResult{
			CommandID: msg.RotateToken.CommandID,
			Status:    domain.CommandStatusCompleted,
			Message:   "agent token rotated",
			Timestamp: now(h.Clock),
		}
		return []domain.AgentMessage{{CommandResult: &result}}, nil
	case msg.ConfigUpdate != nil:
		h.State.ApplyConfigUpdate(*msg.ConfigUpdate)
		return nil, nil
	case msg.Ping != nil:
		snapshot := h.State.Snapshot()
		heartbeat := domain.Heartbeat{
			ClusterID:    snapshot.ClusterID,
			AgentVersion: snapshot.AgentVersion,
			Capabilities: snapshot.Capabilities,
			Timestamp:    now(h.Clock),
		}
		return []domain.AgentMessage{{Heartbeat: &heartbeat}}, nil
	default:
		return nil, nil
	}
}
