package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

type roomMembersInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID whose joined members should be listed"`
}

type roomMemberInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID whose joined member should be fetched"`
	UserID string `json:"user_id" jsonschema:"Matrix user ID to look up within the room"`
}

type roomMembersOutput struct {
	BaseResult
	Members []matrixclient.MemberInfo `json:"members" jsonschema:"Joined members of the room"`
}

type roomMemberOutput struct {
	BaseResult
	Member matrixclient.MemberInfo `json:"member"`
}

func RegisterRoomMembers(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("room.members", "Joined-member lookup tools for specific rooms.")
	if !active.Allows(scopes.ScopeRoomMembersRead) {
		return
	}

	catalog.AddTool(r, "room.members", scopes.ScopeRoomMembersRead, &mcp.Tool{
		Name:        "matrix.v1.room.members.list",
		Description: "List joined members for a room, including display names and avatar URLs when available.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input roomMembersInput) (*mcp.CallToolResult, roomMembersOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, roomMembersOutput{}, err
		}
		members, err := deps.Matrix.ListRoomMembers(ctx, input.RoomID)
		if err != nil {
			return nil, roomMembersOutput{}, err
		}
		return nil, roomMembersOutput{BaseResult: deps.baseResult(), Members: members}, nil
	})

	catalog.AddTool(r, "room.members", scopes.ScopeRoomMembersRead, &mcp.Tool{
		Name:        "matrix.v1.room.members.get",
		Description: "Fetch one joined member from a room by user ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input roomMemberInput) (*mcp.CallToolResult, roomMemberOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, roomMemberOutput{}, err
		}
		if err := requireNonEmpty("user_id", input.UserID); err != nil {
			return nil, roomMemberOutput{}, err
		}
		member, err := deps.Matrix.GetRoomMember(ctx, input.RoomID, input.UserID)
		if err != nil {
			return nil, roomMemberOutput{}, err
		}
		return nil, roomMemberOutput{BaseResult: deps.baseResult(), Member: member}, nil
	})
}
