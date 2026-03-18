package usecase

import (
	"context"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type CollectInventory struct {
	Collector port.InventoryCollector
	Clock     port.Clock
}

func (u CollectInventory) Run(ctx context.Context) (*domain.InventorySnapshot, error) {
	snapshot, err := u.Collector.CollectInventory(ctx)
	if err != nil {
		return nil, err
	}
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = now(u.Clock)
	}
	return snapshot, nil
}
