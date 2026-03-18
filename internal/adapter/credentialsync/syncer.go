package credentialsync

import (
	"context"

	"github.com/gitopshq-io/agent/internal/adapter/kubernetes"
	"github.com/gitopshq-io/agent/internal/domain"
)

type Syncer struct {
	Kubernetes *kubernetes.Client
	Targets    []string
}

func (s Syncer) Sync(ctx context.Context, req domain.CredentialSyncRequest) (domain.CredentialSyncResult, error) {
	return s.Kubernetes.MirrorCredentials(ctx, req, s.Targets)
}
