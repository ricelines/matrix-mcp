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
	if parsed.Allows(ScopeRoomsAliasWrite) || parsed.Allows(ScopeRoomsDirectoryWrite) {
		t.Fatal("default scopes should not enable alias or directory writes")
	}
	if parsed.Allows(ScopeUsersCreate) && !parsed.Allows(ScopeUsersRead) {
		t.Fatal("users.create should not replace baseline user reads")
	}
	if parsed.Allows(ScopeRoomsCreate) || parsed.Allows(ScopeRoomsJoin) || parsed.Allows(ScopeRoomsInvite) || parsed.Allows(ScopeRoomsLeave) {
		t.Fatal("default scopes should not enable room topology mutations")
	}
	if parsed.Allows(ScopeMessagesRedact) {
		t.Fatal("default scopes should not enable redactions")
	}
}

func TestParseAllowsAliasAndDirectoryScopes(t *testing.T) {
	parsed, err := Parse("default,rooms.alias.read,rooms.alias.write,rooms.directory.read,rooms.directory.write")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !parsed.Allows(ScopeRoomsAliasRead) || !parsed.Allows(ScopeRoomsDirectoryRead) {
		t.Fatal("expected alias and directory read scopes to be enabled")
	}
	if !parsed.Allows(ScopeRoomsAliasWrite) || !parsed.Allows(ScopeRoomsDirectoryWrite) {
		t.Fatal("expected alias and directory write scopes to be enabled")
	}
}

func TestParseRejectsUnknownScope(t *testing.T) {
	if _, err := Parse("wat"); err == nil {
		t.Fatal("Parse() unexpectedly succeeded")
	}
}
