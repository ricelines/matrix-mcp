package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/catalog"
	matrixclient "github.com/ricelines/chat/matrix-mcp-go/internal/matrix"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

type roomStateGetInput struct {
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID whose state should be read"`
	EventType string `json:"event_type" jsonschema:"Matrix event type to fetch, such as m.room.create or m.room.topic"`
	StateKey  string `json:"state_key,omitempty" jsonschema:"State key to fetch. Defaults to the empty state key."`
}

type roomStateListInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID whose state should be listed"`
}

type roomStateEventOutput struct {
	BaseResult
	Event matrixclient.EventSummary `json:"event"`
}

type roomStateListOutput struct {
	BaseResult
	Events []matrixclient.EventSummary `json:"events" jsonschema:"State events currently visible in the room"`
}

func RegisterRoomState(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("room.state", "State-event lookup tools for specific rooms.")
	if !active.Allows(scopes.ScopeRoomStateRead) {
		return
	}

	catalog.AddTool(r, "room.state", scopes.ScopeRoomStateRead, &mcp.Tool{
		Name:        "matrix.v1.room.state.get",
		Description: "Fetch one room state event by event type and state key.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input roomStateGetInput) (*mcp.CallToolResult, roomStateEventOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, roomStateEventOutput{}, err
		}
		if err := requireNonEmpty("event_type", input.EventType); err != nil {
			return nil, roomStateEventOutput{}, err
		}
		evt, err := deps.Matrix.GetStateEvent(ctx, input.RoomID, input.EventType, input.StateKey)
		if err != nil {
			return nil, roomStateEventOutput{}, err
		}
		return nil, roomStateEventOutput{BaseResult: deps.baseResult(), Event: evt}, nil
	})

	catalog.AddTool(r, "room.state", scopes.ScopeRoomStateRead, &mcp.Tool{
		Name:        "matrix.v1.room.state.list",
		Description: "List room state events currently visible to the active account.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input roomStateListInput) (*mcp.CallToolResult, roomStateListOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, roomStateListOutput{}, err
		}
		events, err := deps.Matrix.ListStateEvents(ctx, input.RoomID)
		if err != nil {
			return nil, roomStateListOutput{}, err
		}
		return nil, roomStateListOutput{BaseResult: deps.baseResult(), Events: events}, nil
	})
}
