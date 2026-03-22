package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"maunium.net/go/mautrix"
)

func TestCreateUserWithRegistrationToken(t *testing.T) {
	var calls int
	svc := newRegistrationTestService(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/_matrix/client/v3/register" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch calls {
		case 1:
			if payload["auth"] != nil {
				t.Fatalf("unexpected auth in initial request: %#v", payload)
			}
			return jsonResponse(t, r, http.StatusUnauthorized, map[string]any{
				"session": "sess-1",
				"flows":   []map[string]any{{"stages": []string{"m.login.registration_token"}}},
			})
		case 2:
			auth := payload["auth"].(map[string]any)
			if auth["type"] != "m.login.registration_token" || auth["token"] != "invite-token" || auth["session"] != "sess-1" {
				t.Fatalf("unexpected token auth payload: %#v", auth)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{
				"user_id":      "@alice:example.com",
				"device_id":    "DEV1",
				"access_token": "access-token",
			})
		default:
			t.Fatalf("unexpected registration call %d", calls)
			return nil
		}
	})
	svc.registrationToken = "invite-token"
	created, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Username: "alice",
		Password: "wonderland",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.UserID != "@alice:example.com" || created.DeviceID != "DEV1" || created.AccessToken != "access-token" {
		t.Fatalf("unexpected CreateUser() result = %#v", created)
	}
}

func TestCreateUserRequiresConfiguredRegistrationTokenWhenHomeserverDemandsIt(t *testing.T) {
	svc := newRegistrationTestService(t, func(r *http.Request) *http.Response {
		return jsonResponse(t, r, http.StatusUnauthorized, map[string]any{
			"session": "sess-1",
			"flows":   []map[string]any{{"stages": []string{"m.login.registration_token"}}},
		})
	})
	_, err := svc.CreateUser(context.Background(), CreateUserRequest{Username: "alice", Password: "wonderland"})
	if err == nil || err.Error() != "homeserver requires a registration token for account creation, but matrix-mcp was started without one" {
		t.Fatalf("CreateUser() error = %v, want configured registration token requirement", err)
	}
}

func TestCreateUserFallsBackToDummyAuth(t *testing.T) {
	var calls int
	svc := newRegistrationTestService(t, func(r *http.Request) *http.Response {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch calls {
		case 1:
			return jsonResponse(t, r, http.StatusUnauthorized, map[string]any{
				"session": "sess-1",
				"flows":   []map[string]any{{"stages": []string{"m.login.dummy"}}},
			})
		case 2:
			auth := payload["auth"].(map[string]any)
			if auth["type"] != "m.login.dummy" || auth["session"] != "sess-1" {
				t.Fatalf("unexpected dummy auth payload: %#v", auth)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{"user_id": "@alice:example.com"})
		default:
			t.Fatalf("unexpected registration call %d", calls)
			return nil
		}
	})
	created, err := svc.CreateUser(context.Background(), CreateUserRequest{Username: "alice"})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if !created.PasswordGenerated || created.Password == "" {
		t.Fatalf("expected generated password, got %#v", created)
	}
}

func TestRoomAliasOperations(t *testing.T) {
	var calls int
	svc := newClientTestService(t, func(r *http.Request) *http.Response {
		calls++
		if got, want := r.URL.EscapedPath(), "/_matrix/client/v3/directory/room/%23welcome:example.com"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		switch calls {
		case 1:
			if r.Method != http.MethodPut {
				t.Fatalf("method = %s, want PUT", r.Method)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["room_id"] != "!welcome:example.com" {
				t.Fatalf("unexpected alias create payload: %#v", payload)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{})
		case 2:
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{
				"room_id": "!welcome:example.com",
				"servers": []string{"backup.example.com", "example.com"},
			})
		case 3:
			if r.Method != http.MethodDelete {
				t.Fatalf("method = %s, want DELETE", r.Method)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{})
		default:
			t.Fatalf("unexpected alias call %d", calls)
			return nil
		}
	})

	created, err := svc.CreateRoomAlias(context.Background(), CreateRoomAliasRequest{
		RoomAlias: "#welcome:example.com",
		RoomID:    "!welcome:example.com",
	})
	if err != nil {
		t.Fatalf("CreateRoomAlias() error = %v", err)
	}
	if created.RoomAlias != "#welcome:example.com" || created.RoomID != "!welcome:example.com" {
		t.Fatalf("unexpected CreateRoomAlias() result = %#v", created)
	}

	resolved, err := svc.GetRoomAlias(context.Background(), "#welcome:example.com")
	if err != nil {
		t.Fatalf("GetRoomAlias() error = %v", err)
	}
	if resolved.RoomID != "!welcome:example.com" {
		t.Fatalf("unexpected GetRoomAlias() room ID = %#v", resolved)
	}
	if len(resolved.Servers) != 2 || resolved.Servers[0] != "backup.example.com" || resolved.Servers[1] != "example.com" {
		t.Fatalf("unexpected GetRoomAlias() servers = %#v", resolved.Servers)
	}

	deleted, err := svc.DeleteRoomAlias(context.Background(), "#welcome:example.com")
	if err != nil {
		t.Fatalf("DeleteRoomAlias() error = %v", err)
	}
	if deleted.RoomAlias != "#welcome:example.com" {
		t.Fatalf("unexpected DeleteRoomAlias() result = %#v", deleted)
	}
}

func TestRoomDirectoryVisibilityOperations(t *testing.T) {
	var calls int
	svc := newClientTestService(t, func(r *http.Request) *http.Response {
		calls++
		if got, want := r.URL.EscapedPath(), "/_matrix/client/v3/directory/list/room/%21welcome:example.com"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		switch calls {
		case 1:
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{"visibility": RoomDirectoryVisibilityPrivate})
		case 2, 3:
			if r.Method != http.MethodPut {
				t.Fatalf("method = %s, want PUT", r.Method)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			want := RoomDirectoryVisibilityPublic
			if calls == 3 {
				want = RoomDirectoryVisibilityPrivate
			}
			if payload["visibility"] != want {
				t.Fatalf("unexpected directory visibility payload: %#v", payload)
			}
			return jsonResponse(t, r, http.StatusOK, map[string]any{})
		default:
			t.Fatalf("unexpected directory visibility call %d", calls)
			return nil
		}
	})

	visibility, err := svc.GetRoomDirectoryVisibility(context.Background(), "!welcome:example.com")
	if err != nil {
		t.Fatalf("GetRoomDirectoryVisibility() error = %v", err)
	}
	if visibility.Visibility != RoomDirectoryVisibilityPrivate {
		t.Fatalf("unexpected GetRoomDirectoryVisibility() result = %#v", visibility)
	}

	published, err := svc.SetRoomDirectoryVisibility(context.Background(), SetRoomDirectoryVisibilityRequest{
		RoomID:     "!welcome:example.com",
		Visibility: RoomDirectoryVisibilityPublic,
	})
	if err != nil {
		t.Fatalf("SetRoomDirectoryVisibility(public) error = %v", err)
	}
	if published.Visibility != RoomDirectoryVisibilityPublic {
		t.Fatalf("unexpected SetRoomDirectoryVisibility(public) result = %#v", published)
	}

	unpublished, err := svc.SetRoomDirectoryVisibility(context.Background(), SetRoomDirectoryVisibilityRequest{
		RoomID:     "!welcome:example.com",
		Visibility: RoomDirectoryVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("SetRoomDirectoryVisibility(private) error = %v", err)
	}
	if unpublished.Visibility != RoomDirectoryVisibilityPrivate {
		t.Fatalf("unexpected SetRoomDirectoryVisibility(private) result = %#v", unpublished)
	}
}

func newRegistrationTestService(t *testing.T, responder func(*http.Request) *http.Response) *Service {
	t.Helper()
	return &Service{
		homeserverURL: "https://example.com",
		newRegistrationClient: func(homeserverURL string) (*mautrix.Client, error) {
			client, err := mautrix.NewClient(homeserverURL, "", "")
			if err != nil {
				return nil, err
			}
			client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return responder(r), nil
			})}
			return client, nil
		},
	}
}

func newClientTestService(t *testing.T, responder func(*http.Request) *http.Response) *Service {
	t.Helper()
	client, err := mautrix.NewClient("https://example.com", "@bot:example.com", "access-token")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return responder(r), nil
	})}
	return &Service{
		client:        client,
		homeserverURL: "https://example.com",
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(t *testing.T, req *http.Request, status int, payload map[string]any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
}
