package config

import "testing"

func TestFromEnvDefaultsScopesAndListenAddr(t *testing.T) {
	t.Setenv(envHomeserverURL, "http://example.com")
	t.Setenv(envUsername, "bot")
	t.Setenv(envPassword, "secret")
	t.Setenv(envRegistrationToken, " invite-token ")
	t.Setenv(envListenAddr, "")
	t.Setenv(envScopes, "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
	}
	if cfg.RegistrationToken != "invite-token" {
		t.Fatalf("RegistrationToken = %q, want invite-token", cfg.RegistrationToken)
	}
	if got := cfg.Scopes.Names(); len(got) == 0 {
		t.Fatalf("default scopes should not be empty")
	}
}

func TestFromEnvRejectsMissingRequiredFields(t *testing.T) {
	t.Setenv(envHomeserverURL, "")
	t.Setenv(envUsername, "")
	t.Setenv(envPassword, "")

	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv() unexpectedly succeeded")
	}
}
