// Package medialink issues short-lived capability URLs for event attachments,
// so a client that cannot send the MCP bearer token (a browser, curl in
// another tool) can still fetch a file the agent was asked for.
package medialink

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Target struct {
	RoomID  string
	EventID string
}

type entry struct {
	target  Target
	expires time.Time
}

type Store struct {
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	links map[string]entry
}

func NewStore(ttl time.Duration) *Store {
	return &Store{ttl: ttl, now: time.Now, links: make(map[string]entry)}
}

// Create returns a new unguessable token (256 bits) for the target.
func (s *Store) Create(target Target) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.pruneLocked(now)
	expires := now.Add(s.ttl)
	s.links[token] = entry{target: target, expires: expires}
	return token, expires, nil
}

// Lookup resolves a token that has not expired yet. Links stay valid until
// they expire, so a download can be retried.
func (s *Store) Lookup(token string) (Target, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.links[token]
	if !ok || !s.now().Before(e.expires) {
		return Target{}, false
	}
	return e.target, true
}

func (s *Store) pruneLocked(now time.Time) {
	for token, e := range s.links {
		if !now.Before(e.expires) {
			delete(s.links, token)
		}
	}
}
