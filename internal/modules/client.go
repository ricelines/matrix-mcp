package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/catalog"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

type identityOutput struct {
	BaseResult
	UserID        string `json:"user_id,omitempty" jsonschema:"Matrix user ID for the active session"`
	DeviceID      string `json:"device_id,omitempty" jsonschema:"Matrix device ID for the active session"`
	HomeserverURL string `json:"homeserver_url" jsonschema:"Homeserver base URL used by the server"`
}

type statusOutput struct {
	BaseResult
	IsActive bool `json:"is_active" jsonschema:"Whether the Matrix client is logged in and ready"`
}

func RegisterClient(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("client", "Identity and readiness information for the active Matrix session.")

	if active.Allows(scopes.ScopeClientIdentityRead) {
		catalog.AddTool(r, "client", scopes.ScopeClientIdentityRead, &mcp.Tool{
			Name:        "matrix.v1.client.identity.get",
			Description: "Return the active Matrix user, device, and homeserver identity.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, identityOutput, error) {
			identity := deps.Matrix.Identity()
			return nil, identityOutput{
				BaseResult:    deps.baseResult(),
				UserID:        identity.UserID,
				DeviceID:      identity.DeviceID,
				HomeserverURL: identity.HomeserverURL,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeClientStatusRead) {
		catalog.AddTool(r, "client", scopes.ScopeClientStatusRead, &mcp.Tool{
			Name:        "matrix.v1.client.status.get",
			Description: "Return whether the Matrix client session is active and ready.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, statusOutput, error) {
			return nil, statusOutput{
				BaseResult: deps.baseResult(),
				IsActive:   deps.Matrix.IsActive(),
			}, nil
		})
	}
}
