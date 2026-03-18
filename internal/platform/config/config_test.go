package config

import "testing"

func TestLoadSecureDefaults(t *testing.T) {
	t.Setenv("GITOPSHQ_HUB_INSECURE", "")
	t.Setenv("GITOPSHQ_DIRECT_DEPLOY_FORCE_OWNERSHIP", "")
	t.Setenv("GITOPSHQ_IDENTITY_STORE_MODE", "")

	cfg := Load()
	if cfg.Hub.Insecure {
		t.Fatal("expected hub transport to default to TLS verification")
	}
	if cfg.DirectDeploy.ForceOwnership {
		t.Fatal("expected force ownership to default to disabled")
	}
	if cfg.Identity.Mode != "file" {
		t.Fatalf("expected identity store mode %q, got %q", "file", cfg.Identity.Mode)
	}
}
