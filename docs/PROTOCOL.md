# Protocol

The canonical hub-agent contract lives in [`proto/agent/v1/agent.proto`](../proto/agent/v1/agent.proto).

Current transport behavior:

- Unary `Register` swaps a short-lived registration token for a durable agent token.
- Bidirectional `Connect` carries heartbeats, inventory, ArgoCD snapshots, drift reports, command acknowledgements, command results, and credential sync results.
- Hub-to-agent messages carry typed command specs, credential sync bundles, token rotation, config updates, and ping frames.
- Command immutability is represented by a canonical `spec_hash` over the execution payload, required capability, and expiry window.
- Secret-backed values references are expected to carry a digest so the agent can reject mutated values blobs before rendering.

The Go package under `proto/agent/v1` is currently maintained manually so the repo can build without external codegen tooling in constrained environments.
