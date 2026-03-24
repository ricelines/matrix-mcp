package modules

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

type timelineMessagesInput struct {
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID whose timeline should be paginated"`
	From      string `json:"from,omitempty" jsonschema:"Pagination token to continue from"`
	To        string `json:"to,omitempty" jsonschema:"Pagination token bound for the request"`
	Direction string `json:"direction,omitempty" jsonschema:"Timeline direction: 'b' for backward or 'f' for forward. Defaults to 'b'."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum number of timeline events to return"`
}

type timelineEventInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID containing the event"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID to fetch"`
}

type timelineContextInput struct {
	RoomID  string `json:"room_id" jsonschema:"Matrix room ID containing the event"`
	EventID string `json:"event_id" jsonschema:"Matrix event ID whose surrounding context should be fetched"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of events before and after the target event"`
}

type timelineRelationsInput struct {
	RoomID       string `json:"room_id" jsonschema:"Matrix room ID containing the event"`
	EventID      string `json:"event_id" jsonschema:"Matrix event ID whose relations should be listed"`
	RelationType string `json:"relation_type,omitempty" jsonschema:"Optional Matrix relation type such as m.annotation, m.replace, or m.thread"`
	EventType    string `json:"event_type,omitempty" jsonschema:"Optional Matrix event type filter such as m.room.message or m.reaction"`
	Direction    string `json:"direction,omitempty" jsonschema:"Timeline direction: 'b' for backward or 'f' for forward. Defaults to 'b'."`
	From         string `json:"from,omitempty" jsonschema:"Pagination token to continue from"`
	To           string `json:"to,omitempty" jsonschema:"Pagination token bound for the request"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of relations to return"`
	Recurse      bool   `json:"recurse,omitempty" jsonschema:"Whether to recursively include transitive relations when the homeserver supports it"`
}

type timelineMessagesOutput struct {
	BaseResult
	Start  string                      `json:"start,omitempty"`
	End    string                      `json:"end,omitempty"`
	Events []matrixclient.EventSummary `json:"events"`
	State  []matrixclient.EventSummary `json:"state,omitempty"`
}

type timelineEventOutput struct {
	BaseResult
	Event matrixclient.EventSummary `json:"event"`
}

type timelineContextOutput struct {
	BaseResult
	Event        matrixclient.EventSummary   `json:"event"`
	EventsBefore []matrixclient.EventSummary `json:"events_before"`
	EventsAfter  []matrixclient.EventSummary `json:"events_after"`
	State        []matrixclient.EventSummary `json:"state,omitempty"`
	Start        string                      `json:"start,omitempty"`
	End          string                      `json:"end,omitempty"`
}

type timelineRelationsOutput struct {
	BaseResult
	Events         []matrixclient.EventSummary `json:"events"`
	NextBatch      string                      `json:"next_batch,omitempty"`
	PrevBatch      string                      `json:"prev_batch,omitempty"`
	RecursionDepth int                         `json:"recursion_depth,omitempty"`
}

func RegisterTimeline(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("timeline", "Timeline pagination, event lookup, context, and relation inspection.")
	if !active.Allows(scopes.ScopeTimelineRead) {
		return
	}

	catalog.AddTool(r, "timeline", scopes.ScopeTimelineRead, &mcp.Tool{
		Name:        "matrix.v1.timeline.messages.list",
		Description: "Paginate the timeline of a room, returning timeline events and any supporting state included by the homeserver.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input timelineMessagesInput) (*mcp.CallToolResult, timelineMessagesOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, timelineMessagesOutput{}, err
		}
		result, err := deps.Matrix.ListMessages(ctx, matrixclient.ListMessagesRequest{
			RoomID:    input.RoomID,
			From:      input.From,
			To:        input.To,
			Direction: input.Direction,
			Limit:     input.Limit,
		})
		if err != nil {
			return nil, timelineMessagesOutput{}, err
		}
		return nil, timelineMessagesOutput{
			BaseResult: deps.baseResult(),
			Start:      result.Start,
			End:        result.End,
			Events:     result.Events,
			State:      result.State,
		}, nil
	})

	catalog.AddTool(r, "timeline", scopes.ScopeTimelineRead, &mcp.Tool{
		Name:        "matrix.v1.timeline.event.get",
		Description: "Fetch one event from a room by event ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input timelineEventInput) (*mcp.CallToolResult, timelineEventOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, timelineEventOutput{}, err
		}
		if err := requireNonEmpty("event_id", input.EventID); err != nil {
			return nil, timelineEventOutput{}, err
		}
		evt, err := deps.Matrix.GetEvent(ctx, input.RoomID, input.EventID)
		if err != nil {
			return nil, timelineEventOutput{}, err
		}
		return nil, timelineEventOutput{BaseResult: deps.baseResult(), Event: evt}, nil
	})

	catalog.AddTool(r, "timeline", scopes.ScopeTimelineRead, &mcp.Tool{
		Name:        "matrix.v1.timeline.event.context.get",
		Description: "Fetch a target event together with surrounding timeline context and supporting state.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input timelineContextInput) (*mcp.CallToolResult, timelineContextOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, timelineContextOutput{}, err
		}
		if err := requireNonEmpty("event_id", input.EventID); err != nil {
			return nil, timelineContextOutput{}, err
		}
		contextResult, err := deps.Matrix.GetEventContext(ctx, input.RoomID, input.EventID, input.Limit)
		if err != nil {
			return nil, timelineContextOutput{}, err
		}
		return nil, timelineContextOutput{
			BaseResult:   deps.baseResult(),
			Event:        contextResult.Event,
			EventsBefore: contextResult.EventsBefore,
			EventsAfter:  contextResult.EventsAfter,
			State:        contextResult.State,
			Start:        contextResult.Start,
			End:          contextResult.End,
		}, nil
	})

	catalog.AddTool(r, "timeline", scopes.ScopeTimelineRead, &mcp.Tool{
		Name:        "matrix.v1.timeline.relations.list",
		Description: "List relation events connected to a target event, such as reactions or edits.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input timelineRelationsInput) (*mcp.CallToolResult, timelineRelationsOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, timelineRelationsOutput{}, err
		}
		if err := requireNonEmpty("event_id", input.EventID); err != nil {
			return nil, timelineRelationsOutput{}, err
		}
		result, err := deps.Matrix.ListRelations(ctx, matrixclient.ListRelationsRequest{
			RoomID:       input.RoomID,
			EventID:      input.EventID,
			RelationType: input.RelationType,
			EventType:    input.EventType,
			Direction:    input.Direction,
			From:         input.From,
			To:           input.To,
			Limit:        input.Limit,
			Recurse:      input.Recurse,
		})
		if err != nil {
			return nil, timelineRelationsOutput{}, err
		}
		return nil, timelineRelationsOutput{
			BaseResult:     deps.baseResult(),
			Events:         result.Events,
			NextBatch:      result.NextBatch,
			PrevBatch:      result.PrevBatch,
			RecursionDepth: result.RecursionDepth,
		}, nil
	})
}
