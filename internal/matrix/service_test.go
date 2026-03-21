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
