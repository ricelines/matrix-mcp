package config

import "testing"

func TestFromEnvDefaultsScopesAndListenAddr(t *testing.T) {
	t.Setenv(envHomeserverURL, "http://example.com")
	t.Setenv(envUsername, "bot")
	t.Setenv(envPassword, "secret")
	t.Setenv(envRegistrationToken, " invite-token ")
	t.Setenv(envE2EEDBPath, " data/e2ee.db ")
	t.Setenv(envAuthToken, " bearer-secret ")
	t.Setenv(envListenAddr, "")
	t.Setenv(envScopes, "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.AuthToken != "bearer-secret" {
		t.Fatalf("AuthToken = %q, want bearer-secret", cfg.AuthToken)
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
	}
	if cfg.RegistrationToken != "invite-token" {
		t.Fatalf("RegistrationToken = %q, want invite-token", cfg.RegistrationToken)
	}
	if cfg.E2EEDBPath != "data/e2ee.db" {
		t.Fatalf("E2EEDBPath = %q, want data/e2ee.db", cfg.E2EEDBPath)
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

func TestFromEnvRequiresAuthTokenOffLoopback(t *testing.T) {
	cases := []struct {
		name       string
		listenAddr string
		token      string
		allow      string
		wantErr    bool
	}{
		{"all interfaces without token", ":8080", "", "", true},
		{"lan address without token", "192.168.0.10:8080", "", "", true},
		{"all interfaces with token", ":8080", "s3cret", "", false},
		{"loopback v4 without token", "127.0.0.1:8080", "", "", false},
		{"loopback v6 without token", "[::1]:8080", "", "", false},
		{"localhost without token", "localhost:8080", "", "", false},
		{"explicit override", ":8080", "", "true", false},
		{"invalid override", ":8080", "", "maybe", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envHomeserverURL, "http://example.com")
			t.Setenv(envUsername, "bot")
			t.Setenv(envPassword, "secret")
			t.Setenv(envE2EEDBPath, "")
			t.Setenv(envListenAddr, tc.listenAddr)
			t.Setenv(envAuthToken, tc.token)
			t.Setenv(envAllowNoAuth, tc.allow)

			_, err := FromEnv()
			if (err != nil) != tc.wantErr {
				t.Fatalf("FromEnv() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
