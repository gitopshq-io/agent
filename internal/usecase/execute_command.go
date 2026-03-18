package usecase

import (
	"context"
	"fmt"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

type ExecuteCommand struct {
	Executor port.CommandExecutor
	Clock    port.Clock
}

func (u ExecuteCommand) Run(ctx context.Context, cmd domain.ExecuteCommand, capabilities domain.CapabilitySet) (domain.CommandResult, error) {
	if err := cmd.Verify(now(u.Clock)); err != nil {
		return failedCommandResult(cmd.CommandID, err.Error(), u.Clock), nil
	}
	if !capabilities.Has(cmd.RequiredCapability) {
		return failedCommandResult(cmd.CommandID, fmt.Sprintf("command requires capability %q", cmd.RequiredCapability), u.Clock), nil
	}
	result, err := u.Executor.Execute(ctx, cmd)
	if err != nil {
		return domain.CommandResult{}, err
	}
	if result.Timestamp.IsZero() {
		result.Timestamp = now(u.Clock)
	}
	if result.CommandID == "" {
		result.CommandID = cmd.CommandID
	}
	return result, nil
}

func failedCommandResult(commandID, message string, clock port.Clock) domain.CommandResult {
	return domain.CommandResult{
		CommandID: commandID,
		Status:    domain.CommandStatusFailed,
		Error:     message,
		Timestamp: now(clock),
	}
}
