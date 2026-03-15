package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/catalog"
	matrixclient "github.com/ricelines/chat/matrix-mcp-go/internal/matrix"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

type listRoomsOutput struct {
	BaseResult
	Rooms []matrixclient.RoomSummary `json:"rooms" jsonschema:"Joined rooms visible to the active Matrix account"`
}

type roomGetInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID to inspect"`
}

type roomPreviewInput struct {
	Room string   `json:"room" jsonschema:"Room ID or room alias to preview"`
	Via  []string `json:"via,omitempty" jsonschema:"Optional via servers for alias previews"`
}

type roomSummaryOutput struct {
	BaseResult
	Room matrixclient.RoomSummary `json:"room"`
}

type createRoomInput struct {
	Name     string   `json:"name,omitempty" jsonschema:"Optional room name"`
	Topic    string   `json:"topic,omitempty" jsonschema:"Optional room topic"`
	IsPublic bool     `json:"is_public,omitempty" jsonschema:"Whether to create a public room"`
	Invite   []string `json:"invite,omitempty" jsonschema:"Optional user IDs to invite"`
	IsDirect bool     `json:"is_direct,omitempty" jsonschema:"Whether the room should be marked as a DM"`
}

type createRoomOutput struct {
	BaseResult
	RoomID string `json:"room_id" jsonschema:"Created Matrix room ID"`
}

type joinRoomInput struct {
	Room   string   `json:"room" jsonschema:"Room ID or room alias to join"`
	Via    []string `json:"via,omitempty" jsonschema:"Optional via servers for alias joins"`
	Reason string   `json:"reason,omitempty" jsonschema:"Optional join reason"`
}

type joinRoomOutput struct {
	BaseResult
	RoomID string `json:"room_id" jsonschema:"Joined Matrix room ID"`
}

func RegisterRooms(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("rooms", "Room discovery, inspection, creation, and join actions.")

	if active.Allows(scopes.ScopeRoomsRead) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsRead, &mcp.Tool{
			Name:        "matrix.v1.rooms.list",
			Description: "List rooms joined by the active Matrix account, including summary metadata when the homeserver provides it.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, listRoomsOutput, error) {
			rooms, err := deps.Matrix.ListRooms(ctx)
			if err != nil {
				return nil, listRoomsOutput{}, err
			}
			return nil, listRoomsOutput{BaseResult: deps.baseResult(), Rooms: rooms}, nil
		})

		catalog.AddTool(r, "rooms", scopes.ScopeRoomsRead, &mcp.Tool{
			Name:        "matrix.v1.rooms.get",
			Description: "Fetch summary metadata for a joined or otherwise accessible room by room ID.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomGetInput) (*mcp.CallToolResult, roomSummaryOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, roomSummaryOutput{}, err
			}
			room, err := deps.Matrix.GetRoom(ctx, input.RoomID)
			if err != nil {
				return nil, roomSummaryOutput{}, err
			}
			return nil, roomSummaryOutput{BaseResult: deps.baseResult(), Room: room}, nil
		})

		catalog.AddTool(r, "rooms", scopes.ScopeRoomsRead, &mcp.Tool{
			Name:        "matrix.v1.rooms.preview",
			Description: "Preview a room by room ID or alias, optionally routing through via servers for alias resolution.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomPreviewInput) (*mcp.CallToolResult, roomSummaryOutput, error) {
			if err := requireNonEmpty("room", input.Room); err != nil {
				return nil, roomSummaryOutput{}, err
			}
			room, err := deps.Matrix.PreviewRoom(ctx, input.Room, input.Via)
			if err != nil {
				return nil, roomSummaryOutput{}, err
			}
			return nil, roomSummaryOutput{BaseResult: deps.baseResult(), Room: room}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsCreate) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsCreate, &mcp.Tool{
			Name:        "matrix.v1.rooms.create",
			Description: "Create a Matrix room with optional invites and topic metadata.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input createRoomInput) (*mcp.CallToolResult, createRoomOutput, error) {
			result, err := deps.Matrix.CreateRoom(ctx, matrixclient.CreateRoomRequest{
				Name:     input.Name,
				Topic:    input.Topic,
				IsPublic: input.IsPublic,
				Invite:   input.Invite,
				IsDirect: input.IsDirect,
			})
			if err != nil {
				return nil, createRoomOutput{}, err
			}
			return nil, createRoomOutput{BaseResult: deps.baseResult(), RoomID: result.RoomID}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsJoin) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsJoin, &mcp.Tool{
			Name:        "matrix.v1.rooms.join",
			Description: "Join a Matrix room by room ID or room alias.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input joinRoomInput) (*mcp.CallToolResult, joinRoomOutput, error) {
			if err := requireNonEmpty("room", input.Room); err != nil {
				return nil, joinRoomOutput{}, err
			}
			result, err := deps.Matrix.JoinRoom(ctx, matrixclient.JoinRoomRequest{RoomIDOrAlias: input.Room, Via: input.Via, Reason: input.Reason})
			if err != nil {
				return nil, joinRoomOutput{}, err
			}
			return nil, joinRoomOutput{BaseResult: deps.baseResult(), RoomID: result.RoomID}, nil
		})
	}
}
