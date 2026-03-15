package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/catalog"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

type versionsInput struct {
	Freshness string `json:"freshness,omitempty" jsonschema:"Optional freshness hint. Present for discoverability; current implementation always queries the homeserver."`
}

type versionsOutput struct {
	BaseResult
	Versions []string `json:"versions" jsonschema:"Supported Matrix spec versions"`
	Features []string `json:"features" jsonschema:"Enabled unstable feature flags"`
}

type capabilitiesOutput struct {
	BaseResult
	Capabilities map[string]any `json:"capabilities" jsonschema:"Homeserver capability object returned by the Matrix client-server API"`
}

func RegisterServer(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("server", "Homeserver-level metadata and feature discovery.")
	if !active.Allows(scopes.ScopeServerRead) {
		return
	}

	catalog.AddTool(r, "server", scopes.ScopeServerRead, &mcp.Tool{
		Name:        "matrix.v1.server.versions.get",
		Description: "Fetch supported Matrix versions and enabled unstable feature flags from the homeserver.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input versionsInput) (*mcp.CallToolResult, versionsOutput, error) {
		versions, err := deps.Matrix.Versions(ctx)
		if err != nil {
			return nil, versionsOutput{}, err
		}
		return nil, versionsOutput{
			BaseResult: deps.baseResult(),
			Versions:   append([]string{}, versions.Versions...),
			Features:   append([]string{}, versions.Features...),
		}, nil
	})

	catalog.AddTool(r, "server", scopes.ScopeServerRead, &mcp.Tool{
		Name:        "matrix.v1.server.capabilities.get",
		Description: "Fetch the homeserver capabilities object, including room-version and feature support details.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, capabilitiesOutput, error) {
		capabilities, err := deps.Matrix.Capabilities(ctx)
		if err != nil {
			return nil, capabilitiesOutput{}, err
		}
		return nil, capabilitiesOutput{BaseResult: deps.baseResult(), Capabilities: capabilities}, nil
	})
}
