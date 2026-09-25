package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSearchUsersIncludesJoinedRoomMembers(t *testing.T) {
	var joinedRoomsCalls atomic.Int32
	svc := newClientTestService(t, func(r *http.Request) *http.Response {
		switch r.URL.Path {
		case "/_matrix/client/v3/user_directory/search":
			// Synapse leaves bridge ghosts out of the directory; only a regular user is found.
			var req struct {
				SearchTerm string `json:"search_term"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			results := []map[string]any{}
			if strings.Contains("alice", strings.ToLower(req.SearchTerm)) {
				results = append(results, map[string]any{"user_id": "@alice:example.com", "display_name": "Alice"})
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{"results": results, "limited": false})
		case "/_matrix/client/v3/joined_rooms":
			joinedRoomsCalls.Add(1)
			return jsonResponse(t, r, http.StatusOK, map[string]any{
				"joined_rooms": []string{"!group:example.com", "!dm:example.com"},
			})
		case "/_matrix/client/v3/rooms/!dm:example.com/joined_members":
			return jsonResponse(t, r, http.StatusOK, map[string]any{"joined": map[string]any{
				"@bot:example.com":        map[string]any{"display_name": "Me"},
				"@whatsapp_1:example.com": map[string]any{"display_name": "Алексей Иванов (WA)"},
			}})
		case "/_matrix/client/v3/rooms/!group:example.com/joined_members":
			return jsonResponse(t, r, http.StatusOK, map[string]any{"joined": map[string]any{
				"@bot:example.com":        map[string]any{"display_name": "Me"},
				"@alice:example.com":      map[string]any{"display_name": "Alice"},
				"@whatsapp_1:example.com": map[string]any{"display_name": "Алексей Иванов (WA)"},
				"@whatsapp_2:example.com": map[string]any{"display_name": "Лёша Петров (WA)"},
			}})
		default:
			return jsonResponse(t, r, http.StatusNotFound, map[string]any{"errcode": "M_NOT_FOUND"})
		}
	})
	ctx := context.Background()

	got, _, err := svc.SearchUsers(ctx, "алексей", 10)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(got) != 1 || got[0].UserID != "@whatsapp_1:example.com" {
		t.Fatalf("SearchUsers(алексей) = %#v, want only the bridged ghost", got)
	}
	if want := []string{"!dm:example.com", "!group:example.com"}; !slices.Equal(got[0].SharedRoomIDs, want) || got[0].SharedRoomCount != 2 {
		t.Fatalf("shared rooms = %v (%d), want %v smallest first", got[0].SharedRoomIDs, got[0].SharedRoomCount, want)
	}

	got, _, err = svc.SearchUsers(ctx, "Леша", 10)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(got) != 1 || got[0].UserID != "@whatsapp_2:example.com" {
		t.Fatalf("SearchUsers(Леша) = %#v, want ё folded to е", got)
	}

	got, _, err = svc.SearchUsers(ctx, "ali", 10)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(got) != 1 || got[0].UserID != "@alice:example.com" || !slices.Equal(got[0].SharedRoomIDs, []string{"!group:example.com"}) {
		t.Fatalf("SearchUsers(ali) = %#v, want the directory hit once, with its shared room", got)
	}
	if got[0].DisplayName != "Alice" {
		t.Fatalf("directory display_name not read: %#v", got[0])
	}

	got, _, err = svc.SearchUsers(ctx, "Me", 10)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	for _, user := range got {
		if user.UserID == "@bot:example.com" {
			t.Fatalf("SearchUsers(Me) returned the account itself: %#v", got)
		}
	}

	if calls := joinedRoomsCalls.Load(); calls != 1 {
		t.Fatalf("joined_rooms fetched %d times, want 1 (member index cached)", calls)
	}
}

func TestSearchUsersAppliesLimit(t *testing.T) {
	svc := newClientTestService(t, func(r *http.Request) *http.Response {
		switch r.URL.Path {
		case "/_matrix/client/v3/user_directory/search":
			return jsonResponse(t, r, http.StatusOK, map[string]any{"results": []map[string]any{}, "limited": false})
		case "/_matrix/client/v3/joined_rooms":
			return jsonResponse(t, r, http.StatusOK, map[string]any{"joined_rooms": []string{"!room:example.com"}})
		case "/_matrix/client/v3/rooms/!room:example.com/joined_members":
			return jsonResponse(t, r, http.StatusOK, map[string]any{"joined": map[string]any{
				"@a1:example.com": map[string]any{"display_name": "Anna"},
				"@a2:example.com": map[string]any{"display_name": "Andrey"},
				"@a3:example.com": map[string]any{"display_name": "Anton"},
			}})
		default:
			return jsonResponse(t, r, http.StatusNotFound, map[string]any{"errcode": "M_NOT_FOUND"})
		}
	})

	got, limited, err := svc.SearchUsers(context.Background(), "an", 2)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(got) != 2 || !limited {
		t.Fatalf("SearchUsers(limit 2) = %d results, limited=%v; want 2, true", len(got), limited)
	}
	if got[0].DisplayName != "Andrey" || got[1].DisplayName != "Anna" {
		t.Fatalf("results not sorted by display name: %#v", got)
	}
}
