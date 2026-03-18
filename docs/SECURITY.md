# Security

Security defaults for the OSS agent:

- The agent initiates the connection to the hub. No inbound cluster connection is required.
- Registration tokens are one-time and short-lived.
- Durable agent identity is stored outside source control, rotated by hub command, and persisted by the chart in a Kubernetes Secret by default.
- Hub transport verifies TLS by default. Plaintext mode is an explicit local-development override.
- Helm chart RBAC defaults to read-only.
- Direct deploy permissions are explicitly opt-in and broaden RBAC because rendered manifests may target arbitrary Kubernetes resources.
- Direct deploy does not force server-side apply ownership by default; taking field ownership is an explicit opt-in.
- Typed commands are locally verified for capability, expiry, and immutable `spec_hash`; arbitrary shell execution is not part of v1.
- Credential sync is modeled as mirrored Kubernetes Secrets so adapters read only referenced secret material.
- Credential mirroring is still namespace-scoped by `GITOPSHQ_CREDENTIAL_SYNC_TARGETS`; the chart exposes the same allow-list through `credentialSync.targets`.
