package scopes

import "testing"

func TestParseDefaultExpansion(t *testing.T) {
	parsed, err := Parse("default,users.create,messages.react")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !parsed.Allows(ScopeUsersCreate) {
		t.Fatal("expected users.create scope to be enabled")
	}
	if !parsed.Allows(ScopeMessagesReact) {
		t.Fatal("expected messages.react scope to be enabled")
	}
	for _, scope := range defaultScopes {
		if !parsed.Allows(scope) {
			t.Fatalf("expected default scope %q to be enabled", scope)
		}
	}
}

func TestParseRejectsUnknownScope(t *testing.T) {
	if _, err := Parse("wat"); err == nil {
		t.Fatal("Parse() unexpectedly succeeded")
	}
}
