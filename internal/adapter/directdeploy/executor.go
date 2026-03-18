package directdeploy

import (
	"context"
	"fmt"

	helmadapter "github.com/gitopshq-io/agent/internal/adapter/helm"
	kustomizeadapter "github.com/gitopshq-io/agent/internal/adapter/kustomize"
	manifestadapter "github.com/gitopshq-io/agent/internal/adapter/manifest"
	"github.com/gitopshq-io/agent/internal/adapter/render"
	sourceadapter "github.com/gitopshq-io/agent/internal/adapter/source"
	"github.com/gitopshq-io/agent/internal/domain"
)

type Runtime interface {
	ApplyRendered(ctx context.Context, namespace string, manifests []render.Manifest) ([]domain.ResourceRef, error)
	RestartWorkload(ctx context.Context, command domain.RestartWorkloadCommand) error
	ScaleWorkload(ctx context.Context, command domain.ScaleWorkloadCommand) error
	CollectDrift(ctx context.Context) (*domain.DriftReport, error)
}

type SourceLoader interface {
	CheckoutGit(ctx context.Context, source domain.SourceRef) (string, func(), error)
	PullHelmOCI(ctx context.Context, source domain.SourceRef) ([]byte, error)
	ResolveValues(ctx context.Context, ref *domain.ValuesRef) (map[string]any, error)
}

type Executor struct {
	Runtime Runtime
	Sources SourceLoader
}

func (e Executor) Execute(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	switch {
	case cmd.DeployHelmRelease != nil:
		return e.deployHelmRelease(ctx, cmd)
	case cmd.ApplyKustomize != nil:
		return e.applyKustomize(ctx, cmd)
	case cmd.ApplyManifestBundle != nil:
		return e.applyManifestBundle(ctx, cmd)
	case cmd.RestartWorkload != nil:
		return e.restartWorkload(ctx, cmd)
	case cmd.ScaleWorkload != nil:
		return e.scaleWorkload(ctx, cmd)
	case cmd.RunDriftScan != nil:
		return e.runDriftScan(ctx, cmd)
	default:
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     "direct deploy executor cannot handle this command",
		}, nil
	}
}

func (e Executor) deployHelmRelease(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	values, err := e.Sources.ResolveValues(ctx, &cmd.DeployHelmRelease.Values)
	if err != nil {
		return domain.CommandResult{}, err
	}

	var manifests []render.Manifest
	switch cmd.DeployHelmRelease.Source.Type {
	case "helm_oci":
		archive, err := e.Sources.PullHelmOCI(ctx, cmd.DeployHelmRelease.Source)
		if err != nil {
			return domain.CommandResult{}, err
		}
		manifests, err = helmadapter.RenderFromArchive(archive, cmd.DeployHelmRelease.ReleaseName, cmd.DeployHelmRelease.Namespace, values)
		if err != nil {
			return domain.CommandResult{}, err
		}
	case "helm_git":
		root, cleanup, err := e.Sources.CheckoutGit(ctx, cmd.DeployHelmRelease.Source)
		if err != nil {
			return domain.CommandResult{}, err
		}
		defer cleanup()
		chartRoot, err := sourceadapter.ResolvePath(root, cmd.DeployHelmRelease.Source.Path)
		if err != nil {
			return domain.CommandResult{}, err
		}
		files, err := sourceadapter.LoadChartFiles(chartRoot)
		if err != nil {
			return domain.CommandResult{}, err
		}
		manifests, err = helmadapter.RenderFromFiles(files, cmd.DeployHelmRelease.ReleaseName, cmd.DeployHelmRelease.Namespace, values)
		if err != nil {
			return domain.CommandResult{}, err
		}
	default:
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     fmt.Sprintf("unsupported helm source type %q", cmd.DeployHelmRelease.Source.Type),
		}, nil
	}

	applied, err := e.Runtime.ApplyRendered(ctx, cmd.DeployHelmRelease.Namespace, manifests)
	if err != nil {
		return domain.CommandResult{}, err
	}
	return completedResult(cmd.CommandID, fmt.Sprintf("applied %d rendered helm resources", len(applied)), applied), nil
}

func (e Executor) applyKustomize(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	if cmd.ApplyKustomize.Source.Type != "kustomize_git" {
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     fmt.Sprintf("unsupported kustomize source type %q", cmd.ApplyKustomize.Source.Type),
		}, nil
	}
	root, cleanup, err := e.Sources.CheckoutGit(ctx, cmd.ApplyKustomize.Source)
	if err != nil {
		return domain.CommandResult{}, err
	}
	defer cleanup()

	sourceRoot, err := sourceadapter.ResolvePath(root, cmd.ApplyKustomize.Source.Path)
	if err != nil {
		return domain.CommandResult{}, err
	}
	files, err := sourceadapter.LoadFiles(sourceRoot)
	if err != nil {
		return domain.CommandResult{}, err
	}
	manifests, err := kustomizeadapter.Build(files)
	if err != nil {
		return domain.CommandResult{}, err
	}
	applied, err := e.Runtime.ApplyRendered(ctx, cmd.ApplyKustomize.Namespace, manifests)
	if err != nil {
		return domain.CommandResult{}, err
	}
	return completedResult(cmd.CommandID, fmt.Sprintf("applied %d kustomize resources", len(applied)), applied), nil
}

func (e Executor) applyManifestBundle(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	if cmd.ApplyManifestBundle.Source.Type != "manifest_git" {
		return domain.CommandResult{
			CommandID: cmd.CommandID,
			Status:    domain.CommandStatusFailed,
			Error:     fmt.Sprintf("unsupported manifest source type %q", cmd.ApplyManifestBundle.Source.Type),
		}, nil
	}
	root, cleanup, err := e.Sources.CheckoutGit(ctx, cmd.ApplyManifestBundle.Source)
	if err != nil {
		return domain.CommandResult{}, err
	}
	defer cleanup()

	sourceRoot, err := sourceadapter.ResolvePath(root, cmd.ApplyManifestBundle.Source.Path)
	if err != nil {
		return domain.CommandResult{}, err
	}
	files, err := sourceadapter.LoadFiles(sourceRoot)
	if err != nil {
		return domain.CommandResult{}, err
	}
	manifests := manifestadapter.RenderFiles(files)
	applied, err := e.Runtime.ApplyRendered(ctx, cmd.ApplyManifestBundle.Namespace, manifests)
	if err != nil {
		return domain.CommandResult{}, err
	}
	return completedResult(cmd.CommandID, fmt.Sprintf("applied %d manifest resources", len(applied)), applied), nil
}

func (e Executor) restartWorkload(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	if err := e.Runtime.RestartWorkload(ctx, *cmd.RestartWorkload); err != nil {
		return domain.CommandResult{}, err
	}
	return completedResult(cmd.CommandID, "workload restarted", nil), nil
}

func (e Executor) scaleWorkload(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	if err := e.Runtime.ScaleWorkload(ctx, *cmd.ScaleWorkload); err != nil {
		return domain.CommandResult{}, err
	}
	return completedResult(cmd.CommandID, fmt.Sprintf("workload scaled to %d replicas", cmd.ScaleWorkload.Replicas), nil), nil
}

func (e Executor) runDriftScan(ctx context.Context, cmd domain.ExecuteCommand) (domain.CommandResult, error) {
	report, err := e.Runtime.CollectDrift(ctx)
	if err != nil {
		return domain.CommandResult{}, err
	}
	findings := 0
	if report != nil {
		findings = len(report.Findings)
	}
	return completedResult(cmd.CommandID, fmt.Sprintf("drift scan completed with %d findings", findings), report), nil
}

func completedResult(commandID, message string, result any) domain.CommandResult {
	return domain.CommandResult{
		CommandID: commandID,
		Status:    domain.CommandStatusCompleted,
		Message:   message,
		Result:    result,
	}
}
