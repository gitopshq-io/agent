package usecase

import (
	"context"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type ReportDrift struct {
	Collector port.InventoryCollector
	Clock     port.Clock
}

func (u ReportDrift) Run(ctx context.Context) (*domain.DriftReport, error) {
	report, err := u.Collector.CollectDrift(ctx)
	if err != nil {
		return nil, err
	}
	if report.Timestamp.IsZero() {
		report.Timestamp = now(u.Clock)
	}
	return report, nil
}
