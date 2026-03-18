package usecase

import (
	"context"
	"log/slog"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type SessionReporter struct {
	Inventory    CollectInventory
	Drift        ReportDrift
	Applications port.ApplicationObserver
	Clock        port.Clock
}

func (r SessionReporter) StatusMessages(ctx context.Context, snapshot SessionSnapshot) []domain.AgentMessage {
	messages := []domain.AgentMessage{
		{
			Heartbeat: &domain.Heartbeat{
				ClusterID:    snapshot.ClusterID,
				AgentVersion: snapshot.AgentVersion,
				Capabilities: append([]domain.Capability(nil), snapshot.Capabilities...),
				Timestamp:    now(r.Clock),
			},
		},
	}
	if snapshotMsg, err := r.Inventory.Run(ctx); err == nil && snapshotMsg != nil {
		messages = append(messages, domain.AgentMessage{InventorySnapshot: snapshotMsg})
	} else if err != nil {
		slog.Warn("failed to collect cluster inventory", "error", err)
	}
	if r.Applications != nil {
		if apps, err := r.Applications.CollectApplications(ctx); err == nil && apps != nil {
			if apps.Timestamp.IsZero() {
				apps.Timestamp = now(r.Clock)
			}
			slog.Info("collected argocd applications", "count", len(apps.Applications))
			messages = append(messages, domain.AgentMessage{ApplicationStatus: apps})
		} else if err != nil {
			slog.Warn("failed to collect argocd applications", "error", err)
		}
	}
	if drift, err := r.Drift.Run(ctx); err == nil && drift != nil {
		messages = append(messages, domain.AgentMessage{DriftReport: drift})
	} else if err != nil {
		slog.Warn("failed to collect drift report", "error", err)
	}
	return messages
}
