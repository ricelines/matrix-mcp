package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
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

type setDisplayNameInput struct {
	DisplayName string `json:"display_name,omitempty" jsonschema:"Display name to set for the active Matrix account. Empty clears the display name."`
}

type setAvatarURLInput struct {
	AvatarURL string `json:"avatar_url,omitempty" jsonschema:"Matrix content URI to set as the active account avatar. Empty clears the avatar URL."`
}

type setPresenceInput struct {
	Presence  string `json:"presence" jsonschema:"Presence value: online, offline, or unavailable"`
	StatusMsg string `json:"status_msg,omitempty" jsonschema:"Optional status message to publish with the presence update"`
}

type profileWriteOutput struct {
	BaseResult
	UserID      string `json:"user_id,omitempty" jsonschema:"Matrix user ID for the active session"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"Configured display name after the update"`
	AvatarURL   string `json:"avatar_url,omitempty" jsonschema:"Configured avatar URL after the update"`
}

type presenceWriteOutput struct {
	BaseResult
	UserID    string `json:"user_id,omitempty" jsonschema:"Matrix user ID for the active session"`
	Presence  string `json:"presence" jsonschema:"Published presence value"`
	StatusMsg string `json:"status_msg,omitempty" jsonschema:"Published status message"`
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

	if active.Allows(scopes.ScopeClientProfileWrite) {
		catalog.AddTool(r, "client", scopes.ScopeClientProfileWrite, &mcp.Tool{
			Name:        "matrix.v1.client.profile.set_display_name",
			Description: "Set or clear the display name for the active Matrix account.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input setDisplayNameInput) (*mcp.CallToolResult, profileWriteOutput, error) {
			if err := deps.Matrix.SetDisplayName(ctx, input.DisplayName); err != nil {
				return nil, profileWriteOutput{}, err
			}
			return nil, profileWriteOutput{
				BaseResult:  deps.baseResult(),
				UserID:      deps.Matrix.Identity().UserID,
				DisplayName: strings.TrimSpace(input.DisplayName),
			}, nil
		})

		catalog.AddTool(r, "client", scopes.ScopeClientProfileWrite, &mcp.Tool{
			Name:        "matrix.v1.client.profile.set_avatar_url",
			Description: "Set or clear the avatar URL for the active Matrix account.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input setAvatarURLInput) (*mcp.CallToolResult, profileWriteOutput, error) {
			if err := deps.Matrix.SetAvatarURL(ctx, input.AvatarURL); err != nil {
				return nil, profileWriteOutput{}, err
			}
			return nil, profileWriteOutput{
				BaseResult: deps.baseResult(),
				UserID:     deps.Matrix.Identity().UserID,
				AvatarURL:  strings.TrimSpace(input.AvatarURL),
			}, nil
		})
	}

	if active.Allows(scopes.ScopeClientPresenceWrite) {
		catalog.AddTool(r, "client", scopes.ScopeClientPresenceWrite, &mcp.Tool{
			Name:        "matrix.v1.client.presence.set",
			Description: "Set the presence and optional status message for the active Matrix account.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input setPresenceInput) (*mcp.CallToolResult, presenceWriteOutput, error) {
			presence := strings.TrimSpace(input.Presence)
			if err := validatePresence(presence); err != nil {
				return nil, presenceWriteOutput{}, err
			}
			if err := deps.Matrix.SetPresence(ctx, matrixclient.SetPresenceRequest{
				Presence:  presence,
				StatusMsg: input.StatusMsg,
			}); err != nil {
				return nil, presenceWriteOutput{}, err
			}
			return nil, presenceWriteOutput{
				BaseResult: deps.baseResult(),
				UserID:     deps.Matrix.Identity().UserID,
				Presence:   presence,
				StatusMsg:  strings.TrimSpace(input.StatusMsg),
			}, nil
		})
	}
}

func validatePresence(presence string) error {
	switch presence {
	case "online", "offline", "unavailable":
		return nil
	default:
		return fmt.Errorf("presence must be one of online, offline, or unavailable")
	}
}
