package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/catalog"
	matrixclient "github.com/ricelines/chat/matrix-mcp-go/internal/matrix"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

type sendTextInput struct {
	RoomID string `json:"room_id" jsonschema:"Matrix room ID that will receive the message"`
	Body   string `json:"body" jsonschema:"Message body"`
	Notice bool   `json:"notice,omitempty" jsonschema:"Send as m.notice instead of m.text"`
}

type replyTextInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID that will receive the reply"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID being replied to"`
	Body    string `json:"body" jsonschema:"Reply body"`
	Notice  bool   `json:"notice,omitempty" jsonschema:"Send the reply as m.notice instead of m.text"`
}

type editTextInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID containing the original message"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID of the message to edit"`
	Body    string `json:"body" jsonschema:"Replacement message body"`
	Notice  bool   `json:"notice,omitempty" jsonschema:"Send the replacement content as m.notice instead of m.text"`
}

type reactInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID containing the target event"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID to react to"`
	Key     string `json:"key" jsonschema:"Reaction key, usually an emoji"`
}

type redactInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID containing the target event"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID to redact"`
	Reason  string `json:"reason,omitempty" jsonschema:"Optional human-readable redaction reason"`
}

type messageEventOutput struct {
	BaseResult
	EventID string `json:"event_id" jsonschema:"Event ID for the event created by the action"`
}

func RegisterMessages(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("messages", "High-level message composition and mutation tools.")

	if active.Allows(scopes.ScopeMessagesSend) {
		catalog.AddTool(r, "messages", scopes.ScopeMessagesSend, &mcp.Tool{
			Name:        "matrix.v1.messages.send_text",
			Description: "Send a text or notice message into a Matrix room.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input sendTextInput) (*mcp.CallToolResult, messageEventOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("body", input.Body); err != nil {
				return nil, messageEventOutput{}, err
			}
			result, err := deps.Matrix.SendText(ctx, matrixclient.SendTextRequest{RoomID: input.RoomID, Body: input.Body, Notice: input.Notice})
			if err != nil {
				return nil, messageEventOutput{}, err
			}
			return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
		})
	}

	if active.Allows(scopes.ScopeMessagesReply) {
		catalog.AddTool(r, "messages", scopes.ScopeMessagesReply, &mcp.Tool{
			Name:        "matrix.v1.messages.reply_text",
			Description: "Send a textual reply to an existing Matrix event.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input replyTextInput) (*mcp.CallToolResult, messageEventOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("event_id", input.EventID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("body", input.Body); err != nil {
				return nil, messageEventOutput{}, err
			}
			result, err := deps.Matrix.ReplyText(ctx, matrixclient.ReplyTextRequest{RoomID: input.RoomID, EventID: input.EventID, Body: input.Body, Notice: input.Notice})
			if err != nil {
				return nil, messageEventOutput{}, err
			}
			return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
		})
	}

	if active.Allows(scopes.ScopeMessagesEdit) {
		catalog.AddTool(r, "messages", scopes.ScopeMessagesEdit, &mcp.Tool{
			Name:        "matrix.v1.messages.edit_text",
			Description: "Edit an existing Matrix message by sending an m.replace relation.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input editTextInput) (*mcp.CallToolResult, messageEventOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("event_id", input.EventID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("body", input.Body); err != nil {
				return nil, messageEventOutput{}, err
			}
			result, err := deps.Matrix.EditText(ctx, matrixclient.EditTextRequest{RoomID: input.RoomID, EventID: input.EventID, Body: input.Body, Notice: input.Notice})
			if err != nil {
				return nil, messageEventOutput{}, err
			}
			return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
		})
	}

	if active.Allows(scopes.ScopeMessagesReact) {
		catalog.AddTool(r, "messages", scopes.ScopeMessagesReact, &mcp.Tool{
			Name:        "matrix.v1.messages.react",
			Description: "React to an event with an annotation such as an emoji.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input reactInput) (*mcp.CallToolResult, messageEventOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("event_id", input.EventID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("key", input.Key); err != nil {
				return nil, messageEventOutput{}, err
			}
			result, err := deps.Matrix.React(ctx, matrixclient.ReactRequest{RoomID: input.RoomID, EventID: input.EventID, Key: input.Key})
			if err != nil {
				return nil, messageEventOutput{}, err
			}
			return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
		})
	}

	if active.Allows(scopes.ScopeMessagesRedact) {
		catalog.AddTool(r, "messages", scopes.ScopeMessagesRedact, &mcp.Tool{
			Name:        "matrix.v1.messages.redact",
			Description: "Redact an event from a room timeline.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input redactInput) (*mcp.CallToolResult, messageEventOutput, error) {
			if err := requireNonEmpty("room_id", input.RoomID); err != nil {
				return nil, messageEventOutput{}, err
			}
			if err := requireNonEmpty("event_id", input.EventID); err != nil {
				return nil, messageEventOutput{}, err
			}
			result, err := deps.Matrix.Redact(ctx, matrixclient.RedactRequest{RoomID: input.RoomID, EventID: input.EventID, Reason: input.Reason})
			if err != nil {
				return nil, messageEventOutput{}, err
			}
			return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
		})
	}
}
