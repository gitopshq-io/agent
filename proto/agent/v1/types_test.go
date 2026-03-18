package agentv1

import (
	"testing"
	"time"
)

func TestExecuteCommandEnsureSpecHash(t *testing.T) {
	cmd := &ExecuteCommand{
		CommandID: "cmd-1",
		ExpiresAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:  &ArgoSyncCommand{Application: "payments"},
	}

	if err := cmd.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}
	if cmd.SpecHash == "" {
		t.Fatal("expected spec hash to be populated")
	}
	if cmd.RequiredCapability != CapabilityArgoCDWrite {
		t.Fatalf("expected required capability %q, got %q", CapabilityArgoCDWrite, cmd.RequiredCapability)
	}
}

func TestExecuteCommandValidateRequiresSinglePayload(t *testing.T) {
	cmd := &ExecuteCommand{
		CommandID:    "cmd-1",
		ArgoSync:     &ArgoSyncCommand{Application: "payments"},
		RunDriftScan: &RunDriftScanCommand{Scope: "cluster"},
	}

	if err := cmd.Validate(); err == nil {
		t.Fatal("expected validation error when multiple payloads are set")
	}
}

func TestExecuteCommandVerifyRejectsSpecHashMismatch(t *testing.T) {
	cmd := &ExecuteCommand{
		CommandID: "cmd-1",
		ExpiresAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		ArgoSync:  &ArgoSyncCommand{Application: "payments"},
	}
	if err := cmd.EnsureSpecHash(); err != nil {
		t.Fatalf("EnsureSpecHash() error = %v", err)
	}
	cmd.ArgoSync.Application = "billing"

	if err := cmd.Verify(time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected spec hash mismatch to be rejected")
	}
}
