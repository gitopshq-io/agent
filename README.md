# GitOpsHQ Agent

GitOpsHQ Agent is the open-source cluster-side component of GitOpsHQ. It runs in Kubernetes, establishes an outbound gRPC session to the hub, reports cluster state, and executes typed commands after local policy checks.

License: Apache 2.0

## Project Links

- Organization profile: <https://github.com/gitopshq-io>
- Repository: <https://github.com/gitopshq-io/agent>
- Releases: <https://github.com/gitopshq-io/agent/releases>
- Protocol contract: [`proto/agent/v1/agent.proto`](proto/agent/v1/agent.proto)
- Helm chart (OCI): `oci://ghcr.io/gitopshq-io/charts/gitopshq-agent`
- Container image: `ghcr.io/gitopshq-io/agent`
- Security policy: [`SECURITY.md`](SECURITY.md)
- Contribution guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)
- Code of conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
- Third-party notices: [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)

## What It Does Today

- Bootstrap and reconnect flow via `Register` + bidirectional `Connect` (`proto/agent/v1`)
- Heartbeats, inventory snapshots, drift reports, and ArgoCD application status reporting
- Typed command execution (no arbitrary shell):
  - ArgoCD: sync, rollback, delete
  - Direct deploy: Helm release, Kustomize, manifest bundle
  - Kubernetes actions: restart workload, scale workload
  - Diagnostics: drift scan, resource inspection (with optional events/logs)
- Credential sync with mirrored Kubernetes Secret reconciliation
- Hub-driven token rotation and runtime config updates

## Security Defaults

- Outbound-only connection model (no inbound cluster tunnel required)
- TLS verification enabled by default (`tls.insecure=true` is an explicit dev override)
- Local command validation requires capability, `expiresAt`, and immutable `spec_hash`
- Helm chart defaults to read-only RBAC (`rbac.profile=readonly`)
- Durable identity persistence defaults to Kubernetes Secret (`persistence.type=secret`)

## Install with Helm

Use a release version from <https://github.com/gitopshq-io/agent/releases>. Example with the latest tagged release in this repo (`v0.1.7`):

```bash
helm upgrade --install gitopshq-agent oci://ghcr.io/gitopshq-io/charts/gitopshq-agent \
  --namespace gitopshq-system \
  --create-namespace \
  --version latest \
  --set hub.address=hub.gitopshq.io:443 \
  --set registrationToken=replace-me
```

Local development without TLS termination:

```bash
helm upgrade --install gitopshq-agent oci://ghcr.io/gitopshq-io/charts/gitopshq-agent \
  --namespace gitopshq-system \
  --create-namespace \
  --version latest \
  --set hub.address=host.docker.internal:50051 \
  --set tls.insecure=true \
  --set registrationToken=replace-me
```

Key chart values:

- `hub.address`, `hub.statusIntervalSeconds`, `registrationToken`
- `agent.clusterName`, `agent.displayName`, `agent.provider`, `agent.region`, `agent.environment`
- `capabilities.*`, `rbac.profile`
- `argocd.*`
- `credentialSync.mode`, `credentialSync.targets`
- `diagnostics.allowedNamespaces`
- `directDeploy.*`
- `persistence.*`
- `tls.insecure`, `proxy.*`

## Troubleshooting

- Stream logs:
  `kubectl logs -n gitopshq-system deploy/gitopshq-agent -f`
- If persisted identity is missing and `registrationToken` is empty, startup fails by design.
- If `argocd.server` has no scheme, the agent normalizes it to `http://` when `argocd.insecure=true`, otherwise `https://`.

## Local Development

```bash
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

The hub repository keeps a copy of `proto/agent/v1`.

```bash
make sync-proto HUB_REPO=/path/to/gitopshq.io
```
