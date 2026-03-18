package usecase

import (
	"context"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type SyncCredentials struct {
	Syncer port.CredentialSyncer
	Clock  port.Clock
}

func (u SyncCredentials) Run(ctx context.Context, req domain.CredentialSyncRequest) (domain.CredentialSyncResult, error) {
	result, err := u.Syncer.Sync(ctx, req)
	if err != nil {
		return domain.CredentialSyncResult{}, err
	}
	if result.Timestamp.IsZero() {
		result.Timestamp = now(u.Clock)
	}
	return result, nil
}
