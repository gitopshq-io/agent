package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
)

type FileIdentityStore struct {
	Path string
}

func (s FileIdentityStore) Load(_ context.Context) (domain.AgentIdentity, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return domain.AgentIdentity{}, err
	}
	var identity domain.AgentIdentity
	if err := json.Unmarshal(data, &identity); err == nil && identity.AgentToken != "" {
		return identity, nil
	}
	return domain.AgentIdentity{AgentToken: strings.TrimSpace(string(data))}, nil
}

func (s FileIdentityStore) Save(_ context.Context, identity domain.AgentIdentity) error {
	payload, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create identity directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".agent-identity-*")
	if err != nil {
		return fmt.Errorf("create identity temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod identity temp file: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write identity temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close identity temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace identity file: %w", err)
	}
	return nil
}

type CommandRouter struct {
	Argo interface {
		Execute(context.Context, domain.ExecuteCommand) (domain.CommandResult, error)
	}
	Direct interface {
		Execute(context.Context, domain.ExecuteCommand) (domain.CommandResult, error)
	}
}

func (e CommandRouter) Execute(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	switch {
	case cmd.ArgoSync != nil || cmd.ArgoRollback != nil:
		if e.Argo != nil {
			return e.Argo.Execute(ctx, cmd)
		}
	case cmd.DeployHelmRelease != nil ||
		cmd.ApplyKustomize != nil ||
		cmd.ApplyManifestBundle != nil ||
		cmd.RestartWorkload != nil ||
		cmd.ScaleWorkload != nil ||
		cmd.RunDriftScan != nil ||
		cmd.InspectResource != nil:
		if e.Direct != nil {
			return e.Direct.Execute(ctx, cmd)
		}
	}
	return domain.CommandResult{
		CommandID: cmd.CommandID,
		Status:    domain.CommandStatusFailed,
		Error:     "command type is not supported by this build",
		Timestamp: time.Now().UTC(),
	}, nil
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
