# GitOpsHQ Agent

Open-source cluster agent for GitOpsHQ. The agent runs inside a Kubernetes cluster, registers with a token, keeps an outbound gRPC session to the hub, reports cluster and ArgoCD state, and executes typed commands that the hub authorizes.

This repository is Apache 2.0 licensed and is intended to be published independently from the hub repository.

## Current Scope

- Token-based registration and reconnecting session
- Typed `Register` and `Connect` contract under `proto/agent/v1`
- Readiness heartbeat, inventory, ArgoCD application snapshot, drift report
- ArgoCD sync and rollback executor
- Direct deploy adapters for `helm_oci`, `helm_git`, `kustomize_git`, and `manifest_git`
- Kubernetes-backed credential sync with mirrored Secret reconciliation
- Helm chart values aligned with hub address, capabilities, RBAC profile, direct deploy, credential sync, TLS, and proxy settings

## Architecture

- `internal/domain` holds transport-agnostic models and command policy validation.
- `internal/usecase` owns registration, session lifecycle, reporting, and inbound hub message handling.
- `internal/port` defines the interfaces used inward by the use cases.
- `internal/adapter` maps gRPC, ArgoCD, and runtime persistence concerns onto those ports.
- `internal/platform` contains environment-driven bootstrap configuration.

## Security Model

- The hub remains authoritative for RBAC, approvals, and policy decisions.
- The agent still performs local enforcement before execution: command capability, expiry, and immutable `spec_hash` must all pass.
- Durable agent identity is stored locally as token plus cluster identity so reconnects keep emitting stable heartbeats. The Helm chart persists this by default in a Kubernetes Secret.
- Direct deploy permissions are opt-in through chart RBAC and capability flags; the default chart remains read-only.

## Helm Install

```bash
helm upgrade --install gitopshq-agent oci://ghcr.io/gitopshq-io/charts/gitopshq-agent \
  --namespace gitopshq-system \
  --create-namespace \
  --version 0.1.0 \
  --set hub.address=agent.gitopshq.example:443 \
  --set registrationToken=replace-me
```

Tagged releases publish both the container image and the Helm chart to GHCR. The chart reference used by the hub install command is `oci://ghcr.io/gitopshq-io/charts/gitopshq-agent`.

For local development without TLS termination, explicitly opt into plaintext transport:

```bash
helm upgrade --install gitopshq-agent oci://ghcr.io/gitopshq-io/charts/gitopshq-agent \
  --namespace gitopshq-system \
  --create-namespace \
  --version 0.1.0 \
  --set hub.address=host.docker.internal:50051 \
  --set tls.insecure=true \
  --set registrationToken=replace-me
```

Important chart values:

- `hub.address`, `hub.statusIntervalSeconds`
- `agent.clusterName`, `agent.displayName`, `agent.provider`, `agent.region`, `agent.environment`
- `persistence.*`
- `rbac.profile`
- `capabilities.*`
- `credentialSync.mode`, `credentialSync.targets`
- `argocd.*`
- `directDeploy.*`
- `directDeploy.forceOwnership`
- `tls.insecure`
- `proxy.*`

Troubleshooting:

- Agent logs are emitted as JSON to stdout. For a live view:
  `kubectl logs -n gitopshq-system deploy/gitopshq-agent -f`
- ArgoCD collection failures now appear as `failed to collect argocd applications`.
- On startup the agent logs whether ArgoCD integration is enabled, disabled, or missing a token.
- Registration tokens are one-time bootstrap tokens. After the first successful join, upgrades should reuse the persisted agent identity and no longer require `registrationToken`.
- Default persistence mode is `secret`. Use `persistence.type=pvc` if you prefer a volume-backed identity file, or `persistence.enabled=false` for ephemeral dev/test installs.
- Upgrading from older chart revisions that used `emptyDir` requires one final upgrade with a fresh `registrationToken` so the new persistent identity store can be seeded.

## Local Development

```bash
cd agent
go test ./...
go run ./cmd/gitopshq-agent
```

## Developer Workflow

```bash
make test
make lint
make chart-template
make chart-package
make docker-build
```

## Proto Sync

The hub repository keeps a local copy of `proto/agent/v1` so it does not need a module dependency on the agent implementation repo.

To sync the contract into the hub repository:

```bash
make sync-proto HUB_REPO=/path/to/gitopshq.io
```
