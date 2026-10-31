package uploadslot

import (
	"errors"
	"testing"
	"time"
)

func TestSlotLifecycle(t *testing.T) {
	store := NewStore(time.Minute, 0)
	token, _, err := store.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Take(token); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Take(empty) error = %v, want ErrEmpty", err)
	}
	if err := store.Fill(token, Upload{Data: []byte("png"), FileName: "qr.png"}); err != nil {
		t.Fatalf("Fill() error = %v", err)
	}
	if err := store.Fill(token, Upload{Data: []byte("other")}); !errors.Is(err, ErrFilled) {
		t.Fatalf("second Fill() error = %v, want ErrFilled", err)
	}
	up, err := store.Take(token)
	if err != nil || string(up.Data) != "png" || up.FileName != "qr.png" {
		t.Fatalf("Take() = %#v, %v", up, err)
	}
	if _, err := store.Take(token); !errors.Is(err, ErrUnknown) {
		t.Fatalf("Take() after take error = %v, want ErrUnknown", err)
	}
	store.Restore(token, up)
	if again, err := store.Take(token); err != nil || string(again.Data) != "png" {
		t.Fatalf("Take() after Restore = %#v, %v", again, err)
	}
}

func TestSlotExpiresAndCap(t *testing.T) {
	now := time.Unix(1000, 0)
	store := NewStore(time.Minute, 1)
	store.now = func() time.Time { return now }

	token, _, err := store.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, _, err := store.Create(); !errors.Is(err, ErrTooManySlots) {
		t.Fatalf("Create() over cap error = %v, want ErrTooManySlots", err)
	}
	now = now.Add(2 * time.Minute)
	if err := store.Fill(token, Upload{Data: []byte("x")}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("Fill(expired) error = %v, want ErrUnknown", err)
	}
	if _, _, err := store.Create(); err != nil {
		t.Fatalf("Create() after expiry error = %v", err)
	}
}
