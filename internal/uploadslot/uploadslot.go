// Package uploadslot stages files for sending. A client that has a file on
// disk (Claude Code with a shell) asks for a slot, PUTs the bytes to its
// capability URL with curl, and then references the slot when sending, so the
// file never has to pass through the model as base64.
package uploadslot

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

var (
	ErrUnknown      = errors.New("upload slot is unknown or expired")
	ErrFilled       = errors.New("upload slot already holds a file")
	ErrEmpty        = errors.New("nothing was uploaded to this slot yet")
	ErrTooManySlots = errors.New("too many pending upload slots")
)

// Upload is a staged file.
type Upload struct {
	Data     []byte
	MIMEType string
	FileName string
}

type slot struct {
	expires time.Time
	upload  *Upload
}

type Store struct {
	ttl        time.Duration
	maxPending int
	now        func() time.Time
	mu         sync.Mutex
	slots      map[string]*slot
}

func NewStore(ttl time.Duration, maxPending int) *Store {
	return &Store{ttl: ttl, maxPending: maxPending, now: time.Now, slots: make(map[string]*slot)}
}

// Create returns a new unguessable token (256 bits) for an empty slot.
func (s *Store) Create() (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.pruneLocked(now)
	if s.maxPending > 0 && len(s.slots) >= s.maxPending {
		return "", time.Time{}, ErrTooManySlots
	}
	expires := now.Add(s.ttl)
	s.slots[token] = &slot{expires: expires}
	return token, expires, nil
}

// Fill stores the file of a slot. A slot is filled once.
func (s *Store) Fill(token string, upload Upload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, ok := s.lookupLocked(token)
	if !ok {
		return ErrUnknown
	}
	if sl.upload != nil {
		return ErrFilled
	}
	sl.upload = &upload
	return nil
}

// Take returns the staged file and frees the slot.
func (s *Store) Take(token string) (Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, ok := s.lookupLocked(token)
	if !ok {
		return Upload{}, ErrUnknown
	}
	if sl.upload == nil {
		return Upload{}, ErrEmpty
	}
	delete(s.slots, token)
	return *sl.upload, nil
}

// Restore puts a taken file back, so a send that failed can be retried.
func (s *Store) Restore(token string, upload Upload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slots[token] = &slot{expires: s.now().Add(s.ttl), upload: &upload}
}

func (s *Store) lookupLocked(token string) (*slot, bool) {
	sl, ok := s.slots[token]
	if !ok || !s.now().Before(sl.expires) {
		return nil, false
	}
	return sl, true
}

func (s *Store) pruneLocked(now time.Time) {
	for token, sl := range s.slots {
		if !now.Before(sl.expires) {
			delete(s.slots, token)
		}
	}
}
