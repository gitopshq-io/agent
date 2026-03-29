package domain

import (
	"testing"
	"time"

	agentv1 "github.com/gitopshq-io/agent/proto/agent/v1"
)

func TestExecuteCommandEnsureSpecHashIgnoresRuntimeFields(t *testing.T) {
	first := ExecuteCommand{
		CommandID:   "cmd-1",
		RequestedBy: "alice",
		ExpiresAt:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:    &ArgoSyncCommand{Application: "payments"},
	}
	second := ExecuteCommand{
		CommandID:   "cmd-2",
		RequestedBy: "bob",
		ExpiresAt:   first.ExpiresAt,
		ArgoSync:    &ArgoSyncCommand{Application: "payments"},
	}

	if err := first.EnsureSpecHash(); err != nil {
		t.Fatalf("first.EnsureSpecHash() error = %v", err)
	}
	if err := second.EnsureSpecHash(); err != nil {
		t.Fatalf("second.EnsureSpecHash() error = %v", err)
	}
	if first.SpecHash != second.SpecHash {
		t.Fatalf("expected equal spec hashes, got %q and %q", first.SpecHash, second.SpecHash)
	}
}

func TestExecuteCommandVerifyRejectsExpiredCommand(t *testing.T) {
	command := ExecuteCommand{
		CommandID: "cmd-1",
		ExpiresAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:  &ArgoSyncCommand{Application: "payments"},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}

	if err := command.Verify(time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected expired command to be rejected")
	}
}

func TestExecuteCommandSpecDigestMatchesProtoCommand(t *testing.T) {
	expiresAt := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)

	domainCommand := ExecuteCommand{
		CommandID:   "cmd-1",
		ExpiresAt:   expiresAt,
		SpecHash:    "ignored",
		RequestedBy: "alice",
		InspectResource: &InspectResourceCommand{
			Namespace:     "payments",
			Kind:          "Deployment",
			Name:          "api",
			Container:     "app",
			TailLines:     200,
			IncludeEvents: true,
			IncludeLogs:   true,
		},
	}
	protoCommand := &agentv1.ExecuteCommand{
		CommandID:   "cmd-2",
		ExpiresAt:   expiresAt,
		SpecHash:    "ignored",
		RequestedBy: "bob",
		InspectResource: &agentv1.InspectResourceCommand{
			Namespace:     "payments",
			Kind:          "Deployment",
			Name:          "api",
			Container:     "app",
			TailLines:     200,
			IncludeEvents: true,
			IncludeLogs:   true,
		},
	}

	domainDigest, err := domainCommand.SpecDigest()
	if err != nil {
		t.Fatalf("domainCommand.SpecDigest() error = %v", err)
	}
	protoDigest, err := protoCommand.SpecDigest()
	if err != nil {
		t.Fatalf("protoCommand.SpecDigest() error = %v", err)
	}
	if domainDigest != protoDigest {
		t.Fatalf("expected matching digests, got domain=%q proto=%q", domainDigest, protoDigest)
	}
}

func TestExecuteCommandArgoDeleteInfersDeleteCapability(t *testing.T) {
	command := ExecuteCommand{
		CommandID:  "cmd-delete-1",
		ExpiresAt:  time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoDelete: &ArgoDeleteCommand{Application: "payments", Namespace: "argocd"},
	}
	if err := command.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}
	if got, want := command.Kind(), "argo_delete"; got != want {
		t.Fatalf("Kind() = %q, want %q", got, want)
	}
	if got, want := command.RequiredCapability, CapabilityArgoCDDelete; got != want {
		t.Fatalf("RequiredCapability = %q, want %q", got, want)
	}
}
