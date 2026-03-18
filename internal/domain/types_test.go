package domain

import (
	"testing"
	"time"
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
