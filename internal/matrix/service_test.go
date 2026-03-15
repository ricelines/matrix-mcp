package matrix

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
)

func TestCreateUserWithRegistrationToken(t *testing.T) {
	var calls int
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": "sess-1",
				"flows":   []map[string]any{{"stages": []string{"m.login.registration_token"}}},
			})
		case 2:
			auth := payload["auth"].(map[string]any)
			if auth["type"] != "m.login.registration_token" || auth["token"] != "invite-token" || auth["session"] != "sess-1" {
				t.Fatalf("unexpected token auth payload: %#v", auth)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user_id":      "@alice:example.com",
				"device_id":    "DEV1",
				"access_token": "access-token",
			})
		default:
			t.Fatalf("unexpected registration call %d", calls)
		}
	}))
	defer server.Close()

	svc := &Service{homeserverURL: server.URL}
	created, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Username:          "alice",
		Password:          "wonderland",
		RegistrationToken: "invite-token",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.UserID != "@alice:example.com" || created.DeviceID != "DEV1" || created.AccessToken != "access-token" {
		t.Fatalf("unexpected CreateUser() result = %#v", created)
	}
}

func TestCreateUserRequiresRegistrationTokenWhenHomeserverDemandsIt(t *testing.T) {
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session": "sess-1",
			"flows":   []map[string]any{{"stages": []string{"m.login.registration_token"}}},
		})
	}))
	defer server.Close()

	svc := &Service{homeserverURL: server.URL}
	_, err := svc.CreateUser(context.Background(), CreateUserRequest{Username: "alice", Password: "wonderland"})
	if err == nil || err.Error() != "homeserver requires a registration_token for account creation" {
		t.Fatalf("CreateUser() error = %v, want registration_token requirement", err)
	}
}

func TestCreateUserFallsBackToDummyAuth(t *testing.T) {
	var calls int
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch calls {
		case 1:
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": "sess-1",
				"flows":   []map[string]any{{"stages": []string{"m.login.dummy"}}},
			})
		case 2:
			auth := payload["auth"].(map[string]any)
			if auth["type"] != "m.login.dummy" || auth["session"] != "sess-1" {
				t.Fatalf("unexpected dummy auth payload: %#v", auth)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": "@alice:example.com"})
		default:
			t.Fatalf("unexpected registration call %d", calls)
		}
	}))
	defer server.Close()

	svc := &Service{homeserverURL: server.URL}
	created, err := svc.CreateUser(context.Background(), CreateUserRequest{Username: "alice"})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if !created.PasswordGenerated || created.Password == "" {
		t.Fatalf("expected generated password, got %#v", created)
	}
}

type testServer struct {
	URL   string
	Close func()
}

func newIPv4Server(t *testing.T, handler http.Handler) *testServer {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	httpServer := &http.Server{Handler: handler}
	go func() {
		_ = httpServer.Serve(listener)
	}()
	return &testServer{
		URL: "http://" + listener.Addr().String(),
		Close: func() {
			_ = httpServer.Close()
			_ = listener.Close()
		},
	}
}
