package integration

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/config"
	"github.com/ricelines/matrix-mcp/internal/mcpserver"
	"github.com/ricelines/matrix-mcp/internal/scopes"
	"github.com/ricelines/matrix-mcp/internal/testutil/tuwunel"
	"maunium.net/go/mautrix"
)

const integrationRegistrationToken = "invite-only-token"

var (
	sharedHomeserver *tuwunel.Instance
	userCounter      atomic.Uint64
)

func TestMain(m *testing.M) {
	if os.Getenv("MATRIX_MCP_GO_RUN_INTEGRATION") != "1" {
		os.Exit(m.Run())
	}

	inst, err := tuwunel.StartManaged(tuwunel.Options{RegistrationToken: integrationRegistrationToken})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "start shared Tuwunel: %v\n", err)
		os.Exit(1)
	}
	sharedHomeserver = inst

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := sharedHomeserver.WaitUntilReady(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "wait for shared Tuwunel: %v\n", err)
		_ = sharedHomeserver.Close()
		os.Exit(1)
	}

	code := m.Run()
	if err := sharedHomeserver.Close(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "stop shared Tuwunel: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func integrationContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func requireIntegrationHomeserver(t *testing.T) *tuwunel.Instance {
	t.Helper()
	if os.Getenv("MATRIX_MCP_GO_RUN_INTEGRATION") != "1" {
		t.Skip("set MATRIX_MCP_GO_RUN_INTEGRATION=1 to run dockerized Tuwunel integration tests")
	}
	if sharedHomeserver == nil {
		t.Fatal("shared Tuwunel instance is unavailable")
	}
	return sharedHomeserver
}

func registerTestUser(t *testing.T, ctx context.Context, hs *tuwunel.Instance, prefix string) (string, string) {
	t.Helper()
	username := nextUsername(prefix)
	password := username + "-secret"
	if err := hs.RegisterUser(ctx, username, password); err != nil {
		t.Fatalf("register user %s: %v", username, err)
	}
	return username, password
}

func nextUsername(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, userCounter.Add(1))
}

func newIntegrationSession(t *testing.T, ctx context.Context, hs *tuwunel.Instance, username, password, rawScopes string) *mcp.ClientSession {
	t.Helper()

	activeScopes, err := scopes.Parse(rawScopes)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	server, err := mcpserver.NewFromConfig(ctx, config.Config{
		ListenAddr:        ":0",
		HomeserverURL:     hs.HomeserverURL,
		Username:          username,
		Password:          password,
		RegistrationToken: hs.RegistrationToken,
		Scopes:            activeScopes,
	})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}

	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "integration-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	return session
}

func integrationHomeserver(t *testing.T) *tuwunel.Instance {
	t.Helper()
	return requireIntegrationHomeserver(t)
}

func registerUser(t *testing.T, ctx context.Context, hs *tuwunel.Instance, prefix string) (string, string) {
	t.Helper()
	return registerTestUser(t, ctx, hs, prefix)
}

func uniqueCredentials(prefix string) (string, string) {
	username := nextUsername(prefix)
	return username, username + "-secret"
}

func newSession(t *testing.T, ctx context.Context, hs *tuwunel.Instance, username, password, rawScopes string) *mcp.ClientSession {
	t.Helper()
	return newIntegrationSession(t, ctx, hs, username, password, rawScopes)
}

func registerAndLoginUser(t *testing.T, ctx context.Context, hs *tuwunel.Instance, prefix string) (string, string, *mautrix.Client) {
	t.Helper()
	username, password := registerTestUser(t, ctx, hs, prefix)
	client, err := hs.LoginClient(ctx, username, password)
	if err != nil {
		t.Fatalf("login user %s: %v", username, err)
	}
	return username, password, client
}
