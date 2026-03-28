package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
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

type roomAliasInput struct {
	RoomAlias string `json:"room_alias" jsonschema:"Matrix room alias to resolve or delete, for example #welcome:example.com"`
}

type roomAliasCreateInput struct {
	RoomAlias string `json:"room_alias" jsonschema:"Matrix room alias to create, for example #welcome:example.com"`
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID the alias should resolve to"`
}

type roomAliasOutput struct {
	BaseResult
	RoomAlias string   `json:"room_alias" jsonschema:"Matrix room alias"`
	RoomID    string   `json:"room_id,omitempty" jsonschema:"Matrix room ID the alias resolves to"`
	Servers   []string `json:"servers,omitempty" jsonschema:"Candidate servers returned by alias resolution"`
}

type roomDirectoryInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID whose room-directory visibility should be inspected or changed"`
}

type roomDirectoryOutput struct {
	BaseResult
	RoomID     string `json:"room_id" jsonschema:"Matrix room ID"`
	Visibility string `json:"visibility" jsonschema:"Room-directory visibility, usually public or private"`
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

type inviteRoomMemberInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID where the invite should be sent"`
	UserID string `json:"user_id" jsonschema:"Matrix user ID to invite into the room"`
	Reason string `json:"reason,omitempty" jsonschema:"Optional invite reason"`
}

type inviteRoomMemberOutput struct {
	BaseResult
	RoomID string `json:"room_id" jsonschema:"Matrix room ID where the invite was sent"`
	UserID string `json:"user_id" jsonschema:"Matrix user ID that was invited"`
}

type leaveRoomInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID to leave"`
	Reason string `json:"reason,omitempty" jsonschema:"Optional leave reason"`
}

type leaveRoomOutput struct {
	BaseResult
	RoomID string `json:"room_id" jsonschema:"Matrix room ID that was left"`
}

type setTypingInput struct {
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID where the active account typing state should be updated"`
	Typing    bool   `json:"typing" jsonschema:"Whether the active account should be marked as typing"`
	TimeoutMS int64  `json:"timeout_ms,omitempty" jsonschema:"Typing timeout in milliseconds when typing is true. Defaults to 30000."`
}

type setTypingOutput struct {
	BaseResult
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID whose typing state was updated"`
	Typing    bool   `json:"typing" jsonschema:"Published typing state for the active account"`
	TimeoutMS int64  `json:"timeout_ms,omitempty" jsonschema:"Typing timeout in milliseconds when typing is true"`
}

type setReadMarkersInput struct {
	RoomID             string `json:"room_id" jsonschema:"Matrix room ID whose read markers should be updated"`
	ReadEventID        string `json:"read_event_id,omitempty" jsonschema:"Event ID to publish as the public m.read receipt"`
	PrivateReadEventID string `json:"private_read_event_id,omitempty" jsonschema:"Event ID to publish as the private m.read.private receipt"`
	FullyReadEventID   string `json:"fully_read_event_id,omitempty" jsonschema:"Event ID to publish as the m.fully_read marker"`
}

type setReadMarkersOutput struct {
	BaseResult
	RoomID             string `json:"room_id" jsonschema:"Matrix room ID whose read markers were updated"`
	ReadEventID        string `json:"read_event_id,omitempty" jsonschema:"Published public read receipt event ID"`
	PrivateReadEventID string `json:"private_read_event_id,omitempty" jsonschema:"Published private read receipt event ID"`
	FullyReadEventID   string `json:"fully_read_event_id,omitempty" jsonschema:"Published fully-read marker event ID"`
}

const defaultTypingTimeoutMS = 30000

func RegisterRooms(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("rooms", "Room discovery, inspection, local activity, alias, directory, creation, and join actions.")

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

	if active.Allows(scopes.ScopeRoomsAliasRead) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsAliasRead, &mcp.Tool{
			Name:        "matrix.v1.rooms.alias.get",
			Description: "Resolve a room alias to its room ID and candidate routing servers.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomAliasInput) (*mcp.CallToolResult, roomAliasOutput, error) {
			if err := requireNonEmpty("room_alias", input.RoomAlias); err != nil {
				return nil, roomAliasOutput{}, err
			}
			result, err := deps.Matrix.GetRoomAlias(ctx, input.RoomAlias)
			if err != nil {
				return nil, roomAliasOutput{}, err
			}
			return nil, roomAliasOutput{
				BaseResult: deps.baseResult(),
				RoomAlias:  result.RoomAlias,
				RoomID:     result.RoomID,
				Servers:    result.Servers,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsDirectoryRead) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsDirectoryRead, &mcp.Tool{
			Name:        "matrix.v1.rooms.directory.get",
			Description: "Read whether a room is published in the room directory or hidden from it.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomDirectoryInput) (*mcp.CallToolResult, roomDirectoryOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			result, err := deps.Matrix.GetRoomDirectoryVisibility(ctx, input.RoomID)
			if err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			return nil, roomDirectoryOutput{
				BaseResult: deps.baseResult(),
				RoomID:     result.RoomID,
				Visibility: result.Visibility,
			}, nil
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

	if active.Allows(scopes.ScopeRoomsAliasWrite) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsAliasWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.alias.create",
			Description: "Create a room alias that points at an existing room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomAliasCreateInput) (*mcp.CallToolResult, roomAliasOutput, error) {
			if err := requireNonEmpty("room_alias", input.RoomAlias); err != nil {
				return nil, roomAliasOutput{}, err
			}
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, roomAliasOutput{}, err
			}
			result, err := deps.Matrix.CreateRoomAlias(ctx, matrixclient.CreateRoomAliasRequest{
				RoomAlias: input.RoomAlias,
				RoomID:    input.RoomID,
			})
			if err != nil {
				return nil, roomAliasOutput{}, err
			}
			return nil, roomAliasOutput{
				BaseResult: deps.baseResult(),
				RoomAlias:  result.RoomAlias,
				RoomID:     result.RoomID,
			}, nil
		})

		catalog.AddTool(r, "rooms", scopes.ScopeRoomsAliasWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.alias.delete",
			Description: "Delete a room alias.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomAliasInput) (*mcp.CallToolResult, roomAliasOutput, error) {
			if err := requireNonEmpty("room_alias", input.RoomAlias); err != nil {
				return nil, roomAliasOutput{}, err
			}
			result, err := deps.Matrix.DeleteRoomAlias(ctx, input.RoomAlias)
			if err != nil {
				return nil, roomAliasOutput{}, err
			}
			return nil, roomAliasOutput{
				BaseResult: deps.baseResult(),
				RoomAlias:  result.RoomAlias,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsDirectoryWrite) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsDirectoryWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.directory.publish",
			Description: "Publish a room into the homeserver room directory.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomDirectoryInput) (*mcp.CallToolResult, roomDirectoryOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			result, err := deps.Matrix.SetRoomDirectoryVisibility(ctx, matrixclient.SetRoomDirectoryVisibilityRequest{
				RoomID:     input.RoomID,
				Visibility: matrixclient.RoomDirectoryVisibilityPublic,
			})
			if err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			return nil, roomDirectoryOutput{
				BaseResult: deps.baseResult(),
				RoomID:     result.RoomID,
				Visibility: result.Visibility,
			}, nil
		})

		catalog.AddTool(r, "rooms", scopes.ScopeRoomsDirectoryWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.directory.unpublish",
			Description: "Hide a room from the homeserver room directory.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input roomDirectoryInput) (*mcp.CallToolResult, roomDirectoryOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			result, err := deps.Matrix.SetRoomDirectoryVisibility(ctx, matrixclient.SetRoomDirectoryVisibilityRequest{
				RoomID:     input.RoomID,
				Visibility: matrixclient.RoomDirectoryVisibilityPrivate,
			})
			if err != nil {
				return nil, roomDirectoryOutput{}, err
			}
			return nil, roomDirectoryOutput{
				BaseResult: deps.baseResult(),
				RoomID:     result.RoomID,
				Visibility: result.Visibility,
			}, nil
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

	if active.Allows(scopes.ScopeRoomsInvite) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsInvite, &mcp.Tool{
			Name:        "matrix.v1.rooms.invite",
			Description: "Invite a Matrix user into an existing room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input inviteRoomMemberInput) (*mcp.CallToolResult, inviteRoomMemberOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, inviteRoomMemberOutput{}, err
			}
			if err := requireNonEmpty("user_id", input.UserID); err != nil {
				return nil, inviteRoomMemberOutput{}, err
			}
			result, err := deps.Matrix.InviteRoomMember(ctx, matrixclient.InviteRoomMemberRequest{
				RoomID: input.RoomID,
				UserID: input.UserID,
				Reason: input.Reason,
			})
			if err != nil {
				return nil, inviteRoomMemberOutput{}, err
			}
			return nil, inviteRoomMemberOutput{
				BaseResult: deps.baseResult(),
				RoomID:     result.RoomID,
				UserID:     result.UserID,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsLeave) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsLeave, &mcp.Tool{
			Name:        "matrix.v1.rooms.leave",
			Description: "Leave a Matrix room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input leaveRoomInput) (*mcp.CallToolResult, leaveRoomOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, leaveRoomOutput{}, err
			}
			result, err := deps.Matrix.LeaveRoom(ctx, matrixclient.LeaveRoomRequest{
				RoomID: input.RoomID,
				Reason: input.Reason,
			})
			if err != nil {
				return nil, leaveRoomOutput{}, err
			}
			return nil, leaveRoomOutput{
				BaseResult: deps.baseResult(),
				RoomID:     result.RoomID,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsTypingWrite) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsTypingWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.typing.set",
			Description: "Set or clear the typing state for the active account in a room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input setTypingInput) (*mcp.CallToolResult, setTypingOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, setTypingOutput{}, err
			}
			timeoutMS := input.TimeoutMS
			if input.Typing && timeoutMS <= 0 {
				timeoutMS = defaultTypingTimeoutMS
			}
			if !input.Typing {
				timeoutMS = 0
			}
			if err := deps.Matrix.SetTyping(ctx, matrixclient.SetTypingRequest{
				RoomID:    input.RoomID,
				Typing:    input.Typing,
				TimeoutMS: timeoutMS,
			}); err != nil {
				return nil, setTypingOutput{}, err
			}
			return nil, setTypingOutput{
				BaseResult: deps.baseResult(),
				RoomID:     input.RoomID,
				Typing:     input.Typing,
				TimeoutMS:  timeoutMS,
			}, nil
		})
	}

	if active.Allows(scopes.ScopeRoomsReadMarkersWrite) {
		catalog.AddTool(r, "rooms", scopes.ScopeRoomsReadMarkersWrite, &mcp.Tool{
			Name:        "matrix.v1.rooms.read_markers.set",
			Description: "Set public or private read receipts and fully-read markers for the active account in a room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input setReadMarkersInput) (*mcp.CallToolResult, setReadMarkersOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, setReadMarkersOutput{}, err
			}
			if input.ReadEventID == "" && input.PrivateReadEventID == "" && input.FullyReadEventID == "" {
				return nil, setReadMarkersOutput{}, requireNonEmpty("read_event_id, private_read_event_id, or fully_read_event_id", "")
			}
			if err := deps.Matrix.SetReadMarkers(ctx, matrixclient.SetReadMarkersRequest{
				RoomID:             input.RoomID,
				ReadEventID:        input.ReadEventID,
				PrivateReadEventID: input.PrivateReadEventID,
				FullyReadEventID:   input.FullyReadEventID,
			}); err != nil {
				return nil, setReadMarkersOutput{}, err
			}
			return nil, setReadMarkersOutput{
				BaseResult:         deps.baseResult(),
				RoomID:             input.RoomID,
				ReadEventID:        input.ReadEventID,
				PrivateReadEventID: input.PrivateReadEventID,
				FullyReadEventID:   input.FullyReadEventID,
			}, nil
		})
	}
}
