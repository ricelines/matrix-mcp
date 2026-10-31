package medialink

import (
	"testing"
	"time"
)

func TestStoreCreateLookupExpire(t *testing.T) {
	now := time.Unix(1_000, 0)
	store := NewStore(10 * time.Minute)
	store.now = func() time.Time { return now }

	target := Target{RoomID: "!room:example.com", EventID: "$event"}
	token, expires, err := store.Create(target)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("token %q is too short to be unguessable", token)
	}
	if !expires.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expires = %v", expires)
	}

	other, _, err := store.Create(target)
	if err != nil || other == token {
		t.Fatalf("second Create() = %q, %v; want a distinct token", other, err)
	}

	got, ok := store.Lookup(token)
	if !ok || got != target {
		t.Fatalf("Lookup() = %#v, %v", got, ok)
	}
	if _, ok := store.Lookup("nope"); ok {
		t.Fatal("Lookup(unknown) succeeded")
	}

	now = now.Add(10 * time.Minute)
	if _, ok := store.Lookup(token); ok {
		t.Fatal("Lookup() succeeded after expiry")
	}
	if _, _, err := store.Create(target); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, ok := store.links[token]; ok {
		t.Fatal("expired link was not pruned")
	}
}
