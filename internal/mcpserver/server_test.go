package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

type fakeMatrix struct {
	identity          matrixclient.Identity
	active            bool
	versions          matrixclient.VersionInfo
	capabilities      map[string]any
	registerAvailable map[string]bool
	created           matrixclient.CreateUserResult
	createErr         error
	searchResults     []matrixclient.SearchUser
	searchLimited     bool
	profileByUserID   map[string]matrixclient.UserProfile
	rooms             []matrixclient.RoomSummary
	roomByID          map[string]matrixclient.RoomSummary
	roomPreviewByKey  map[string]matrixclient.RoomSummary
	roomAliasByAlias  map[string]matrixclient.RoomAliasResult
	roomDirectoryByID map[string]matrixclient.RoomDirectoryVisibilityResult
	membersByRoom     map[string][]matrixclient.MemberInfo
	stateEventsByRoom map[string][]matrixclient.EventSummary
	eventByKey        map[string]matrixclient.EventSummary
	contextByKey      map[string]matrixclient.EventContextResult
	relationsByKey    map[string]matrixclient.RelationsResult
	createdRoom       matrixclient.CreateRoomResult
	joinedRoom        matrixclient.JoinRoomResult
	sentMessage       matrixclient.EventWriteResult
	replyMessage      matrixclient.EventWriteResult
	editMessage       matrixclient.EventWriteResult
	reactionMessage   matrixclient.EventWriteResult
	redactionMessage  matrixclient.EventWriteResult
	lastCreateUser    matrixclient.CreateUserRequest
	lastDisplayName   string
	lastAvatarURL     string
	lastPresence      matrixclient.SetPresenceRequest
	lastCreateRoom    matrixclient.CreateRoomRequest
	lastJoinRoom      matrixclient.JoinRoomRequest
	lastInviteRoom    matrixclient.InviteRoomMemberRequest
	lastLeaveRoom     matrixclient.LeaveRoomRequest
	lastTyping        matrixclient.SetTypingRequest
	lastReadMarkers   matrixclient.SetReadMarkersRequest
	lastCreateAlias   matrixclient.CreateRoomAliasRequest
	lastDeleteAlias   string
	lastDirectoryGet  string
	lastDirectorySet  matrixclient.SetRoomDirectoryVisibilityRequest
	lastSendText      matrixclient.SendTextRequest
	lastReplyText     matrixclient.ReplyTextRequest
	lastEditText      matrixclient.EditTextRequest
	lastReact         matrixclient.ReactRequest
	lastRedact        matrixclient.RedactRequest
	lastPreviewRoom   struct {
		Room string
		Via  []string
	}
	lastMessages  matrixclient.ListMessagesRequest
	lastRelations matrixclient.ListRelationsRequest
}

func (f *fakeMatrix) Identity() matrixclient.Identity { return f.identity }
func (f *fakeMatrix) IsActive() bool                  { return f.active }
func (f *fakeMatrix) Versions(ctx context.Context) (matrixclient.VersionInfo, error) {
	return f.versions, nil
}
func (f *fakeMatrix) Capabilities(ctx context.Context) (map[string]any, error) {
	return f.capabilities, nil
}
func (f *fakeMatrix) RegisterAvailable(ctx context.Context, username string) (bool, error) {
	available, ok := f.registerAvailable[username]
	if !ok {
		return false, fmt.Errorf("unknown username %s", username)
	}
	return available, nil
}
func (f *fakeMatrix) CreateUser(ctx context.Context, req matrixclient.CreateUserRequest) (matrixclient.CreateUserResult, error) {
	f.lastCreateUser = req
	if f.createErr != nil {
		return matrixclient.CreateUserResult{}, f.createErr
	}
	return f.created, nil
}
func (f *fakeMatrix) SearchUsers(ctx context.Context, query string, limit int) ([]matrixclient.SearchUser, bool, error) {
	return f.searchResults, f.searchLimited, nil
}
func (f *fakeMatrix) GetProfile(ctx context.Context, userID string) (matrixclient.UserProfile, error) {
	profile, ok := f.profileByUserID[userID]
	if !ok {
		return matrixclient.UserProfile{}, errors.New("not found")
	}
	return profile, nil
}
func (f *fakeMatrix) SetDisplayName(ctx context.Context, displayName string) error {
	f.lastDisplayName = displayName
	return nil
}
func (f *fakeMatrix) SetAvatarURL(ctx context.Context, avatarURL string) error {
	f.lastAvatarURL = avatarURL
	return nil
}
func (f *fakeMatrix) SetPresence(ctx context.Context, req matrixclient.SetPresenceRequest) error {
	f.lastPresence = req
	return nil
}
func (f *fakeMatrix) ListRooms(ctx context.Context) ([]matrixclient.RoomSummary, error) {
	return f.rooms, nil
}
func (f *fakeMatrix) GetRoom(ctx context.Context, roomID string) (matrixclient.RoomSummary, error) {
	room, ok := f.roomByID[roomID]
	if !ok {
		return matrixclient.RoomSummary{}, errors.New("not found")
	}
	return room, nil
}
func (f *fakeMatrix) PreviewRoom(ctx context.Context, room string, via []string) (matrixclient.RoomSummary, error) {
	f.lastPreviewRoom.Room = room
	f.lastPreviewRoom.Via = append([]string{}, via...)
	key := room + "|" + strings.Join(via, ",")
	preview, ok := f.roomPreviewByKey[key]
	if !ok {
		return matrixclient.RoomSummary{}, errors.New("not found")
	}
	return preview, nil
}
func (f *fakeMatrix) ListRoomMembers(ctx context.Context, roomID string) ([]matrixclient.MemberInfo, error) {
	members, ok := f.membersByRoom[roomID]
	if !ok {
		return nil, errors.New("not found")
	}
	return members, nil
}
func (f *fakeMatrix) GetRoomMember(ctx context.Context, roomID string, userID string) (matrixclient.MemberInfo, error) {
	members, ok := f.membersByRoom[roomID]
	if !ok {
		return matrixclient.MemberInfo{}, errors.New("not found")
	}
	for _, member := range members {
		if member.UserID == userID {
			return member, nil
		}
	}
	return matrixclient.MemberInfo{}, errors.New("not found")
}
func (f *fakeMatrix) GetStateEvent(ctx context.Context, roomID string, eventType string, stateKey string) (matrixclient.EventSummary, error) {
	key := roomID + "|" + eventType + "|" + stateKey
	state, ok := f.eventByKey[key]
	if !ok {
		return matrixclient.EventSummary{}, errors.New("not found")
	}
	return state, nil
}
func (f *fakeMatrix) ListStateEvents(ctx context.Context, roomID string) ([]matrixclient.EventSummary, error) {
	state, ok := f.stateEventsByRoom[roomID]
	if !ok {
		return nil, errors.New("not found")
	}
	return state, nil
}
func (f *fakeMatrix) ListMessages(ctx context.Context, req matrixclient.ListMessagesRequest) (matrixclient.MessagesResult, error) {
	f.lastMessages = req
	result, ok := f.contextMessages(req.RoomID)
	if !ok {
		return matrixclient.MessagesResult{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) GetEvent(ctx context.Context, roomID string, eventID string) (matrixclient.EventSummary, error) {
	key := roomID + "|" + eventID
	result, ok := f.eventByKey[key]
	if !ok {
		return matrixclient.EventSummary{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) GetEventContext(ctx context.Context, roomID string, eventID string, limit int) (matrixclient.EventContextResult, error) {
	key := roomID + "|" + eventID
	result, ok := f.contextByKey[key]
	if !ok {
		return matrixclient.EventContextResult{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) ListRelations(ctx context.Context, req matrixclient.ListRelationsRequest) (matrixclient.RelationsResult, error) {
	f.lastRelations = req
	key := req.RoomID + "|" + req.EventID + "|" + req.RelationType + "|" + req.EventType
	result, ok := f.relationsByKey[key]
	if !ok {
		return matrixclient.RelationsResult{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) CreateRoom(ctx context.Context, req matrixclient.CreateRoomRequest) (matrixclient.CreateRoomResult, error) {
	f.lastCreateRoom = req
	return f.createdRoom, nil
}
func (f *fakeMatrix) JoinRoom(ctx context.Context, req matrixclient.JoinRoomRequest) (matrixclient.JoinRoomResult, error) {
	f.lastJoinRoom = req
	return f.joinedRoom, nil
}
func (f *fakeMatrix) InviteRoomMember(ctx context.Context, req matrixclient.InviteRoomMemberRequest) (matrixclient.InviteRoomMemberResult, error) {
	f.lastInviteRoom = req
	return matrixclient.InviteRoomMemberResult{RoomID: req.RoomID, UserID: req.UserID}, nil
}
func (f *fakeMatrix) LeaveRoom(ctx context.Context, req matrixclient.LeaveRoomRequest) (matrixclient.LeaveRoomResult, error) {
	f.lastLeaveRoom = req
	return matrixclient.LeaveRoomResult{RoomID: req.RoomID}, nil
}
func (f *fakeMatrix) SetTyping(ctx context.Context, req matrixclient.SetTypingRequest) error {
	f.lastTyping = req
	return nil
}
func (f *fakeMatrix) SetReadMarkers(ctx context.Context, req matrixclient.SetReadMarkersRequest) error {
	f.lastReadMarkers = req
	return nil
}
func (f *fakeMatrix) CreateRoomAlias(ctx context.Context, req matrixclient.CreateRoomAliasRequest) (matrixclient.CreateRoomAliasResult, error) {
	f.lastCreateAlias = req
	return matrixclient.CreateRoomAliasResult{RoomAlias: req.RoomAlias, RoomID: req.RoomID}, nil
}
func (f *fakeMatrix) GetRoomAlias(ctx context.Context, roomAlias string) (matrixclient.RoomAliasResult, error) {
	result, ok := f.roomAliasByAlias[roomAlias]
	if !ok {
		return matrixclient.RoomAliasResult{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) DeleteRoomAlias(ctx context.Context, roomAlias string) (matrixclient.DeleteRoomAliasResult, error) {
	f.lastDeleteAlias = roomAlias
	return matrixclient.DeleteRoomAliasResult{RoomAlias: roomAlias}, nil
}
func (f *fakeMatrix) GetRoomDirectoryVisibility(ctx context.Context, roomID string) (matrixclient.RoomDirectoryVisibilityResult, error) {
	f.lastDirectoryGet = roomID
	result, ok := f.roomDirectoryByID[roomID]
	if !ok {
		return matrixclient.RoomDirectoryVisibilityResult{}, errors.New("not found")
	}
	return result, nil
}
func (f *fakeMatrix) SetRoomDirectoryVisibility(ctx context.Context, req matrixclient.SetRoomDirectoryVisibilityRequest) (matrixclient.RoomDirectoryVisibilityResult, error) {
	f.lastDirectorySet = req
	return matrixclient.RoomDirectoryVisibilityResult{RoomID: req.RoomID, Visibility: req.Visibility}, nil
}
func (f *fakeMatrix) SendText(ctx context.Context, req matrixclient.SendTextRequest) (matrixclient.EventWriteResult, error) {
	f.lastSendText = req
	return f.sentMessage, nil
}
func (f *fakeMatrix) ReplyText(ctx context.Context, req matrixclient.ReplyTextRequest) (matrixclient.EventWriteResult, error) {
	f.lastReplyText = req
	return f.replyMessage, nil
}
func (f *fakeMatrix) EditText(ctx context.Context, req matrixclient.EditTextRequest) (matrixclient.EventWriteResult, error) {
	f.lastEditText = req
	return f.editMessage, nil
}
func (f *fakeMatrix) React(ctx context.Context, req matrixclient.ReactRequest) (matrixclient.EventWriteResult, error) {
	f.lastReact = req
	return f.reactionMessage, nil
}
func (f *fakeMatrix) Redact(ctx context.Context, req matrixclient.RedactRequest) (matrixclient.EventWriteResult, error) {
	f.lastRedact = req
	return f.redactionMessage, nil
}

func (f *fakeMatrix) contextMessages(roomID string) (matrixclient.MessagesResult, bool) {
	key := roomID + "|messages"
	result, ok := f.contextByKey[key]
	if !ok {
		return matrixclient.MessagesResult{}, false
	}
	return matrixclient.MessagesResult{Start: result.Start, End: result.End, Events: result.EventsBefore, State: result.State}, true
}

func connectTestSession(t *testing.T, server *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Raw().Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func structuredMap(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return decoded
}

func TestScopeFiltering(t *testing.T) {
	backend := &fakeMatrix{}
	server := New(backend, scopes.Default())
	session := connectTestSession(t, server)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true
	}
	if !seen["matrix.v1.timeline.messages.list"] {
		t.Fatal("timeline read tool missing from default scope set")
	}
	if !seen["matrix.v1.rooms.alias.get"] || !seen["matrix.v1.rooms.directory.get"] {
		t.Fatal("room alias/directory read tools missing from default scope set")
	}
	if seen["matrix.v1.users.create"] {
		t.Fatal("users.create should not be available without users.create scope")
	}
	if !seen["matrix.v1.messages.send_text"] || !seen["matrix.v1.messages.reply_text"] || !seen["matrix.v1.messages.edit_text"] || !seen["matrix.v1.messages.react"] {
		t.Fatal("core messaging tools missing from default scope set")
	}
	if seen["matrix.v1.client.profile.set_display_name"] || seen["matrix.v1.client.presence.set"] || seen["matrix.v1.rooms.typing.set"] || seen["matrix.v1.rooms.read_markers.set"] {
		t.Fatal("safe opt-in tools should not be available without the safe scope set")
	}
	if seen["matrix.v1.rooms.invite"] {
		t.Fatal("rooms.invite should not be available without rooms.invite scope")
	}
	if seen["matrix.v1.rooms.leave"] {
		t.Fatal("rooms.leave should not be available without rooms.leave scope")
	}
	if seen["matrix.v1.messages.redact"] {
		t.Fatal("messages.redact should not be available without messages.redact scope")
	}
	if seen["matrix.v1.rooms.alias.create"] || seen["matrix.v1.rooms.directory.publish"] {
		t.Fatal("room alias/directory write tools should not be available without their write scopes")
	}
}

func TestAliasAndDirectoryScopeFiltering(t *testing.T) {
	backend := &fakeMatrix{}
	active, err := scopes.Parse("default,rooms.alias.read,rooms.alias.write,rooms.directory.read,rooms.directory.write")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	server := New(backend, active)
	session := connectTestSession(t, server)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true
	}
	if !seen["matrix.v1.rooms.alias.get"] || !seen["matrix.v1.rooms.directory.get"] {
		t.Fatal("room alias/directory read tools missing despite explicit read scopes")
	}
	if !seen["matrix.v1.rooms.alias.create"] || !seen["matrix.v1.rooms.alias.delete"] {
		t.Fatal("room alias write tools missing despite explicit alias write scope")
	}
	if !seen["matrix.v1.rooms.directory.publish"] || !seen["matrix.v1.rooms.directory.unpublish"] {
		t.Fatal("room directory write tools missing despite explicit directory write scope")
	}
	if seen["matrix.v1.rooms.create"] {
		t.Fatal("rooms.create should not be available without rooms.create scope")
	}
}

func TestSafeScopeFiltering(t *testing.T) {
	backend := &fakeMatrix{}
	active, err := scopes.Parse("default,safe")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	server := New(backend, active)
	session := connectTestSession(t, server)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true
	}
	if !seen["matrix.v1.client.profile.set_display_name"] || !seen["matrix.v1.client.profile.set_avatar_url"] || !seen["matrix.v1.client.presence.set"] {
		t.Fatal("client safe tools missing despite explicit safe scope set")
	}
	if !seen["matrix.v1.rooms.typing.set"] || !seen["matrix.v1.rooms.read_markers.set"] {
		t.Fatal("room safe tools missing despite explicit safe scope set")
	}
	if seen["matrix.v1.rooms.create"] || seen["matrix.v1.messages.redact"] {
		t.Fatal("safe scope set should not unlock privileged room or message mutation tools")
	}
}

func TestRecursiveDiscoveryResources(t *testing.T) {
	backend := &fakeMatrix{}
	active, err := scopes.Parse("default,users.create,messages.react")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	root, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "matrix://modules"})
	if err != nil {
		t.Fatalf("ReadResource(modules) error = %v", err)
	}
	if got := root.Contents[0].Text; !containsAll(got, "matrix://module/users", "matrix://module/timeline", "matrix://module/room.state") {
		t.Fatalf("unexpected root resource body: %s", got)
	}

	module, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "matrix://module/timeline"})
	if err != nil {
		t.Fatalf("ReadResource(module/timeline) error = %v", err)
	}
	if got := module.Contents[0].Text; !containsAll(got, "matrix.v1.timeline.messages.list", "matrix://tool/matrix.v1.timeline.messages.list") {
		t.Fatalf("unexpected timeline module body: %s", got)
	}

	tool, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "matrix://tool/matrix.v1.messages.react"})
	if err != nil {
		t.Fatalf("ReadResource(tool/messages.react) error = %v", err)
	}
	if got := tool.Contents[0].Text; !containsAll(got, "event_id", "tools/call") {
		t.Fatalf("unexpected tool detail body: %s", got)
	}
}

func TestDiscoveryResourcesStayShallowAndDiscoveredToolsCanBeCalledDirectly(t *testing.T) {
	backend := &fakeMatrix{
		reactionMessage: matrixclient.EventWriteResult{EventID: "$reaction"},
	}
	active, err := scopes.Parse("default,messages.react")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	root, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "matrix://modules"})
	if err != nil {
		t.Fatalf("ReadResource(modules) error = %v", err)
	}
	rootText := root.Contents[0].Text
	if !containsAll(rootText, "matrix://module/messages", "matrix://module/rooms", "matrix://module/timeline") {
		t.Fatalf("unexpected root resource body: %s", rootText)
	}
	if strings.Contains(rootText, "matrix://tool/") {
		t.Fatalf("root resource should not inline tool detail links: %s", rootText)
	}
	if strings.Contains(rootText, "## Input schema") {
		t.Fatalf("root resource should not inline schemas: %s", rootText)
	}

	module, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "matrix://module/messages"})
	if err != nil {
		t.Fatalf("ReadResource(module/messages) error = %v", err)
	}
	moduleText := module.Contents[0].Text
	if !containsAll(moduleText, "matrix.v1.messages.react", "matrix://tool/matrix.v1.messages.react") {
		t.Fatalf("unexpected module resource body: %s", moduleText)
	}
	if strings.Contains(moduleText, "## Input schema") || strings.Contains(moduleText, "## Output schema") {
		t.Fatalf("module resource should stay summary-level, not inline tool schemas: %s", moduleText)
	}
	if strings.Contains(moduleText, "matrix://module/") {
		t.Fatalf("module resource should list tools, not point to a deeper discovery tree: %s", moduleText)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "matrix.v1.messages.react",
		Arguments: map[string]any{
			"room_id":  "!joined:example.com",
			"event_id": "$event",
			"key":      "eyes",
		},
	})
	if err != nil {
		t.Fatalf("messages.react error = %v", err)
	}
	if structuredMap(t, result)["event_id"] != "$reaction" {
		t.Fatalf("unexpected messages.react payload = %#v", structuredMap(t, result))
	}
	if backend.lastReact.RoomID != "!joined:example.com" || backend.lastReact.EventID != "$event" || backend.lastReact.Key != "eyes" {
		t.Fatalf("unexpected direct tool call request = %#v", backend.lastReact)
	}
}

func TestClientAndServerTools(t *testing.T) {
	backend := &fakeMatrix{
		identity:     matrixclient.Identity{UserID: "@bot:example.com", DeviceID: "DEVICE", HomeserverURL: "http://example.com"},
		active:       true,
		versions:     matrixclient.VersionInfo{Versions: []string{"v1.15", "v1.16"}, Features: []string{"org.matrix.msc3916.stable"}},
		capabilities: map[string]any{"capabilities": map[string]any{"m.room_versions": map[string]any{"default": "10"}}},
	}
	server := New(backend, scopes.Default())
	session := connectTestSession(t, server)
	ctx := context.Background()

	identity, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.client.identity.get"})
	if err != nil {
		t.Fatalf("identity tool error = %v", err)
	}
	identityPayload := structuredMap(t, identity)
	if identityPayload["user_id"] != "@bot:example.com" {
		t.Fatalf("user_id = %v", identityPayload["user_id"])
	}

	status, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.client.status.get"})
	if err != nil {
		t.Fatalf("status tool error = %v", err)
	}
	if structuredMap(t, status)["is_active"] != true {
		t.Fatalf("unexpected status payload = %#v", structuredMap(t, status))
	}

	versions, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.server.versions.get"})
	if err != nil {
		t.Fatalf("versions tool error = %v", err)
	}
	if got := len(structuredMap(t, versions)["versions"].([]any)); got != 2 {
		t.Fatalf("versions len = %d", got)
	}

	caps, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.server.capabilities.get"})
	if err != nil {
		t.Fatalf("capabilities tool error = %v", err)
	}
	if structuredMap(t, caps)["capabilities"] == nil {
		t.Fatalf("unexpected capabilities payload = %#v", structuredMap(t, caps))
	}
}

func TestClientSafeTools(t *testing.T) {
	active, err := scopes.Parse("default,safe")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{
		identity: matrixclient.Identity{UserID: "@bot:example.com", DeviceID: "DEVICE", HomeserverURL: "http://example.com"},
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	displayName, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "matrix.v1.client.profile.set_display_name",
		Arguments: map[string]any{"display_name": "Release Bot"},
	})
	if err != nil {
		t.Fatalf("client.profile.set_display_name error = %v", err)
	}
	if structuredMap(t, displayName)["display_name"] != "Release Bot" || backend.lastDisplayName != "Release Bot" {
		t.Fatalf("unexpected display name payload / request = %#v / %q", structuredMap(t, displayName), backend.lastDisplayName)
	}

	avatar, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "matrix.v1.client.profile.set_avatar_url",
		Arguments: map[string]any{"avatar_url": "mxc://example.com/bot"},
	})
	if err != nil {
		t.Fatalf("client.profile.set_avatar_url error = %v", err)
	}
	if structuredMap(t, avatar)["avatar_url"] != "mxc://example.com/bot" || backend.lastAvatarURL != "mxc://example.com/bot" {
		t.Fatalf("unexpected avatar payload / request = %#v / %q", structuredMap(t, avatar), backend.lastAvatarURL)
	}

	presence, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "matrix.v1.client.presence.set",
		Arguments: map[string]any{"presence": "unavailable", "status_msg": "triaging"},
	})
	if err != nil {
		t.Fatalf("client.presence.set error = %v", err)
	}
	if structuredMap(t, presence)["presence"] != "unavailable" || backend.lastPresence.Presence != "unavailable" || backend.lastPresence.StatusMsg != "triaging" {
		t.Fatalf("unexpected presence payload / request = %#v / %#v", structuredMap(t, presence), backend.lastPresence)
	}
}

func TestUsersTools(t *testing.T) {
	active, err := scopes.Parse("default,users.create")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{
		registerAvailable: map[string]bool{"alice": true},
		created: matrixclient.CreateUserResult{
			UserID:            "@alice:example.com",
			DeviceID:          "ALICEDEV",
			AccessToken:       "token",
			Password:          "generated-secret",
			PasswordGenerated: true,
		},
		searchResults: []matrixclient.SearchUser{{UserID: "@alice:example.com", DisplayName: "Alice"}},
		searchLimited: false,
		profileByUserID: map[string]matrixclient.UserProfile{
			"@alice:example.com": {UserID: "@alice:example.com", DisplayName: "Alice", AvatarURL: "mxc://avatar"},
		},
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	available, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.users.register_available", Arguments: map[string]any{"username": "alice"}})
	if err != nil {
		t.Fatalf("register_available error = %v", err)
	}
	if structuredMap(t, available)["available"] != true {
		t.Fatalf("available payload = %#v", structuredMap(t, available))
	}

	created, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.users.create", Arguments: map[string]any{"username": "alice"}})
	if err != nil {
		t.Fatalf("users.create error = %v", err)
	}
	createdPayload := structuredMap(t, created)
	if createdPayload["user_id"] != "@alice:example.com" {
		t.Fatalf("created payload = %#v", createdPayload)
	}
	if backend.lastCreateUser.Username != "alice" {
		t.Fatalf("lastCreateUser = %#v", backend.lastCreateUser)
	}

	seached, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.users.search", Arguments: map[string]any{"query": "ali", "limit": 5}})
	if err != nil {
		t.Fatalf("users.search error = %v", err)
	}
	results := structuredMap(t, seached)["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("search results len = %d", len(results))
	}

	profile, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.users.profile.get", Arguments: map[string]any{"user_id": "@alice:example.com"}})
	if err != nil {
		t.Fatalf("users.profile.get error = %v", err)
	}
	profilePayload := structuredMap(t, profile)
	profileMap := profilePayload["profile"].(map[string]any)
	if profileMap["display_name"] != "Alice" {
		t.Fatalf("profile payload = %#v", profilePayload)
	}
}

func TestUsersCreateRejectsFullUserID(t *testing.T) {
	active, err := scopes.Parse("default,users.create")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	server := New(&fakeMatrix{}, active)
	session := connectTestSession(t, server)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "matrix.v1.users.create", Arguments: map[string]any{"username": "@alice:example.com"}})
	if err != nil {
		t.Fatalf("users.create transport error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("users.create result = %#v, want localpart validation error", result)
	}
}

func TestRoomReadAndWriteTools(t *testing.T) {
	active, err := scopes.Parse("default,rooms.create,rooms.join")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{
		rooms: []matrixclient.RoomSummary{{RoomID: "!joined:example.com", Name: "Joined"}},
		roomByID: map[string]matrixclient.RoomSummary{
			"!joined:example.com": {RoomID: "!joined:example.com", Name: "Joined"},
		},
		roomPreviewByKey: map[string]matrixclient.RoomSummary{
			"#public:example.com|example.com": {RoomID: "!public:example.com", Name: "Public"},
		},
		membersByRoom: map[string][]matrixclient.MemberInfo{
			"!joined:example.com": {{UserID: "@alice:example.com", DisplayName: "Alice"}},
		},
		stateEventsByRoom: map[string][]matrixclient.EventSummary{
			"!joined:example.com": {{EventID: "$state", Type: "m.room.create", RoomID: "!joined:example.com"}},
		},
		eventByKey: map[string]matrixclient.EventSummary{
			"!joined:example.com|m.room.create|": {EventID: "$state", Type: "m.room.create", RoomID: "!joined:example.com"},
			"!joined:example.com|$event":         {EventID: "$event", Type: "m.room.message", RoomID: "!joined:example.com"},
		},
		contextByKey: map[string]matrixclient.EventContextResult{
			"!joined:example.com|$event": {
				Event:        matrixclient.EventSummary{EventID: "$event", Type: "m.room.message"},
				EventsBefore: []matrixclient.EventSummary{{EventID: "$before"}},
				EventsAfter:  []matrixclient.EventSummary{{EventID: "$after"}},
				State:        []matrixclient.EventSummary{{EventID: "$state"}},
				Start:        "start-token",
				End:          "end-token",
			},
			"!joined:example.com|messages": {
				EventsBefore: []matrixclient.EventSummary{{EventID: "$event"}},
				State:        []matrixclient.EventSummary{{EventID: "$state"}},
				Start:        "start-token",
				End:          "end-token",
			},
		},
		relationsByKey: map[string]matrixclient.RelationsResult{
			"!joined:example.com|$event|m.annotation|m.reaction": {
				Events:         []matrixclient.EventSummary{{EventID: "$reaction"}},
				NextBatch:      "next-token",
				RecursionDepth: 1,
			},
		},
		createdRoom: matrixclient.CreateRoomResult{RoomID: "!created:example.com"},
		joinedRoom:  matrixclient.JoinRoomResult{RoomID: "!joined:example.com"},
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	rooms, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.list"})
	if err != nil {
		t.Fatalf("rooms.list error = %v", err)
	}
	if len(structuredMap(t, rooms)["rooms"].([]any)) != 1 {
		t.Fatalf("unexpected rooms.list payload = %#v", structuredMap(t, rooms))
	}

	room, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.get", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("rooms.get error = %v", err)
	}
	if structuredMap(t, room)["room"].(map[string]any)["room_id"] != "!joined:example.com" {
		t.Fatalf("unexpected rooms.get payload = %#v", structuredMap(t, room))
	}

	preview, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.preview", Arguments: map[string]any{"room": "#public:example.com", "via": []string{"example.com"}}})
	if err != nil {
		t.Fatalf("rooms.preview error = %v", err)
	}
	if structuredMap(t, preview)["room"].(map[string]any)["room_id"] != "!public:example.com" {
		t.Fatalf("unexpected rooms.preview payload = %#v", structuredMap(t, preview))
	}

	members, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.room.members.list", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("room.members.list error = %v", err)
	}
	if len(structuredMap(t, members)["members"].([]any)) != 1 {
		t.Fatalf("unexpected room.members.list payload = %#v", structuredMap(t, members))
	}

	member, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.room.members.get", Arguments: map[string]any{"room_id": "!joined:example.com", "user_id": "@alice:example.com"}})
	if err != nil {
		t.Fatalf("room.members.get error = %v", err)
	}
	if structuredMap(t, member)["member"].(map[string]any)["display_name"] != "Alice" {
		t.Fatalf("unexpected room.members.get payload = %#v", structuredMap(t, member))
	}

	stateGet, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.room.state.get", Arguments: map[string]any{"room_id": "!joined:example.com", "event_type": "m.room.create"}})
	if err != nil {
		t.Fatalf("room.state.get error = %v", err)
	}
	if structuredMap(t, stateGet)["event"].(map[string]any)["event_id"] != "$state" {
		t.Fatalf("unexpected room.state.get payload = %#v", structuredMap(t, stateGet))
	}

	stateList, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.room.state.list", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("room.state.list error = %v", err)
	}
	if len(structuredMap(t, stateList)["events"].([]any)) != 1 {
		t.Fatalf("unexpected room.state.list payload = %#v", structuredMap(t, stateList))
	}

	messages, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.timeline.messages.list", Arguments: map[string]any{"room_id": "!joined:example.com", "direction": "f", "limit": 5}})
	if err != nil {
		t.Fatalf("timeline.messages.list error = %v", err)
	}
	if structuredMap(t, messages)["start"] != "start-token" || backend.lastMessages.Direction != "f" {
		t.Fatalf("unexpected timeline.messages.list payload = %#v / %#v", structuredMap(t, messages), backend.lastMessages)
	}

	eventGet, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.timeline.event.get", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event"}})
	if err != nil {
		t.Fatalf("timeline.event.get error = %v", err)
	}
	if structuredMap(t, eventGet)["event"].(map[string]any)["event_id"] != "$event" {
		t.Fatalf("unexpected timeline.event.get payload = %#v", structuredMap(t, eventGet))
	}

	contextResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.timeline.event.context.get", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event", "limit": 5}})
	if err != nil {
		t.Fatalf("timeline.event.context.get error = %v", err)
	}
	if len(structuredMap(t, contextResult)["events_before"].([]any)) != 1 {
		t.Fatalf("unexpected timeline.event.context.get payload = %#v", structuredMap(t, contextResult))
	}

	relations, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.timeline.relations.list", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event", "relation_type": "m.annotation", "event_type": "m.reaction"}})
	if err != nil {
		t.Fatalf("timeline.relations.list error = %v", err)
	}
	if len(structuredMap(t, relations)["events"].([]any)) != 1 {
		t.Fatalf("unexpected timeline.relations.list payload = %#v", structuredMap(t, relations))
	}

	created, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.create", Arguments: map[string]any{"name": "General", "is_public": true, "invite": []string{"@alice:example.com"}}})
	if err != nil {
		t.Fatalf("rooms.create error = %v", err)
	}
	if structuredMap(t, created)["room_id"] != "!created:example.com" || !backend.lastCreateRoom.IsPublic {
		t.Fatalf("unexpected rooms.create payload / request = %#v / %#v", structuredMap(t, created), backend.lastCreateRoom)
	}

	joined, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.join", Arguments: map[string]any{"room": "#public:example.com", "via": []string{"example.com"}}})
	if err != nil {
		t.Fatalf("rooms.join error = %v", err)
	}
	if structuredMap(t, joined)["room_id"] != "!joined:example.com" || backend.lastJoinRoom.RoomIDOrAlias != "#public:example.com" {
		t.Fatalf("unexpected rooms.join payload / request = %#v / %#v", structuredMap(t, joined), backend.lastJoinRoom)
	}
}

func TestRoomInviteTool(t *testing.T) {
	active, err := scopes.Parse("default,rooms.invite")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	invited, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "matrix.v1.rooms.invite",
		Arguments: map[string]any{
			"room_id": "!joined:example.com",
			"user_id": "@bot:example.com",
			"reason":  "onboarding",
		},
	})
	if err != nil {
		t.Fatalf("rooms.invite error = %v", err)
	}
	payload := structuredMap(t, invited)
	if payload["room_id"] != "!joined:example.com" || payload["user_id"] != "@bot:example.com" {
		t.Fatalf("unexpected rooms.invite payload = %#v", payload)
	}
	if backend.lastInviteRoom.RoomID != "!joined:example.com" || backend.lastInviteRoom.UserID != "@bot:example.com" || backend.lastInviteRoom.Reason != "onboarding" {
		t.Fatalf("unexpected rooms.invite request = %#v", backend.lastInviteRoom)
	}
}

func TestRoomLeaveTool(t *testing.T) {
	active, err := scopes.Parse("default,rooms.leave")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	left, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "matrix.v1.rooms.leave",
		Arguments: map[string]any{
			"room_id": "!joined:example.com",
			"reason":  "handoff complete",
		},
	})
	if err != nil {
		t.Fatalf("rooms.leave error = %v", err)
	}
	payload := structuredMap(t, left)
	if payload["room_id"] != "!joined:example.com" {
		t.Fatalf("unexpected rooms.leave payload = %#v", payload)
	}
	if backend.lastLeaveRoom.RoomID != "!joined:example.com" || backend.lastLeaveRoom.Reason != "handoff complete" {
		t.Fatalf("unexpected rooms.leave request = %#v", backend.lastLeaveRoom)
	}
}

func TestRoomSafeTools(t *testing.T) {
	active, err := scopes.Parse("default,safe")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	typing, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "matrix.v1.rooms.typing.set",
		Arguments: map[string]any{"room_id": "!joined:example.com", "typing": true},
	})
	if err != nil {
		t.Fatalf("rooms.typing.set error = %v", err)
	}
	if structuredMap(t, typing)["timeout_ms"] != float64(30000) || backend.lastTyping.TimeoutMS != 30000 {
		t.Fatalf("unexpected rooms.typing.set payload / request = %#v / %#v", structuredMap(t, typing), backend.lastTyping)
	}

	readMarkers, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "matrix.v1.rooms.read_markers.set",
		Arguments: map[string]any{
			"room_id":               "!joined:example.com",
			"read_event_id":         "$public",
			"private_read_event_id": "$private",
			"fully_read_event_id":   "$full",
		},
	})
	if err != nil {
		t.Fatalf("rooms.read_markers.set error = %v", err)
	}
	payload := structuredMap(t, readMarkers)
	if payload["fully_read_event_id"] != "$full" || backend.lastReadMarkers.PrivateReadEventID != "$private" {
		t.Fatalf("unexpected rooms.read_markers.set payload / request = %#v / %#v", payload, backend.lastReadMarkers)
	}
}

func TestRoomAliasAndDirectoryTools(t *testing.T) {
	active, err := scopes.Parse("default,rooms.alias.read,rooms.alias.write,rooms.directory.read,rooms.directory.write")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{
		roomAliasByAlias: map[string]matrixclient.RoomAliasResult{
			"#welcome:example.com": {
				RoomAlias: "#welcome:example.com",
				RoomID:    "!joined:example.com",
				Servers:   []string{"backup.example.com", "example.com"},
			},
		},
		roomDirectoryByID: map[string]matrixclient.RoomDirectoryVisibilityResult{
			"!joined:example.com": {RoomID: "!joined:example.com", Visibility: matrixclient.RoomDirectoryVisibilityPrivate},
		},
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	alias, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.alias.get", Arguments: map[string]any{"room_alias": "#welcome:example.com"}})
	if err != nil {
		t.Fatalf("rooms.alias.get error = %v", err)
	}
	aliasPayload := structuredMap(t, alias)
	if aliasPayload["room_id"] != "!joined:example.com" {
		t.Fatalf("unexpected rooms.alias.get payload = %#v", aliasPayload)
	}
	if len(aliasPayload["servers"].([]any)) != 2 {
		t.Fatalf("unexpected rooms.alias.get servers = %#v", aliasPayload)
	}

	directory, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.directory.get", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("rooms.directory.get error = %v", err)
	}
	if structuredMap(t, directory)["visibility"] != matrixclient.RoomDirectoryVisibilityPrivate || backend.lastDirectoryGet != "!joined:example.com" {
		t.Fatalf("unexpected rooms.directory.get payload / request = %#v / %q", structuredMap(t, directory), backend.lastDirectoryGet)
	}

	createdAlias, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.alias.create", Arguments: map[string]any{"room_alias": "#welcome:example.com", "room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("rooms.alias.create error = %v", err)
	}
	if structuredMap(t, createdAlias)["room_id"] != "!joined:example.com" || backend.lastCreateAlias.RoomAlias != "#welcome:example.com" {
		t.Fatalf("unexpected rooms.alias.create payload / request = %#v / %#v", structuredMap(t, createdAlias), backend.lastCreateAlias)
	}

	deletedAlias, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.alias.delete", Arguments: map[string]any{"room_alias": "#welcome:example.com"}})
	if err != nil {
		t.Fatalf("rooms.alias.delete error = %v", err)
	}
	if structuredMap(t, deletedAlias)["room_alias"] != "#welcome:example.com" || backend.lastDeleteAlias != "#welcome:example.com" {
		t.Fatalf("unexpected rooms.alias.delete payload / request = %#v / %q", structuredMap(t, deletedAlias), backend.lastDeleteAlias)
	}

	published, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.directory.publish", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("rooms.directory.publish error = %v", err)
	}
	if structuredMap(t, published)["visibility"] != matrixclient.RoomDirectoryVisibilityPublic || backend.lastDirectorySet.Visibility != matrixclient.RoomDirectoryVisibilityPublic {
		t.Fatalf("unexpected rooms.directory.publish payload / request = %#v / %#v", structuredMap(t, published), backend.lastDirectorySet)
	}

	unpublished, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.rooms.directory.unpublish", Arguments: map[string]any{"room_id": "!joined:example.com"}})
	if err != nil {
		t.Fatalf("rooms.directory.unpublish error = %v", err)
	}
	if structuredMap(t, unpublished)["visibility"] != matrixclient.RoomDirectoryVisibilityPrivate || backend.lastDirectorySet.Visibility != matrixclient.RoomDirectoryVisibilityPrivate {
		t.Fatalf("unexpected rooms.directory.unpublish payload / request = %#v / %#v", structuredMap(t, unpublished), backend.lastDirectorySet)
	}
}

func TestMessageMutationTools(t *testing.T) {
	active, err := scopes.Parse("default,messages.redact")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	backend := &fakeMatrix{
		sentMessage:      matrixclient.EventWriteResult{EventID: "$sent"},
		replyMessage:     matrixclient.EventWriteResult{EventID: "$reply"},
		editMessage:      matrixclient.EventWriteResult{EventID: "$edit"},
		reactionMessage:  matrixclient.EventWriteResult{EventID: "$reaction"},
		redactionMessage: matrixclient.EventWriteResult{EventID: "$redaction"},
	}
	server := New(backend, active)
	session := connectTestSession(t, server)
	ctx := context.Background()

	sent, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.messages.send_text", Arguments: map[string]any{"room_id": "!joined:example.com", "body": "hello", "notice": true}})
	if err != nil {
		t.Fatalf("messages.send_text error = %v", err)
	}
	if structuredMap(t, sent)["event_id"] != "$sent" || !backend.lastSendText.Notice {
		t.Fatalf("unexpected send_text payload / request = %#v / %#v", structuredMap(t, sent), backend.lastSendText)
	}

	replied, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.messages.reply_text", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event", "body": "reply"}})
	if err != nil {
		t.Fatalf("messages.reply_text error = %v", err)
	}
	if structuredMap(t, replied)["event_id"] != "$reply" || backend.lastReplyText.EventID != "$event" {
		t.Fatalf("unexpected reply_text payload / request = %#v / %#v", structuredMap(t, replied), backend.lastReplyText)
	}

	edited, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.messages.edit_text", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event", "body": "edit"}})
	if err != nil {
		t.Fatalf("messages.edit_text error = %v", err)
	}
	if structuredMap(t, edited)["event_id"] != "$edit" || backend.lastEditText.Body != "edit" {
		t.Fatalf("unexpected edit_text payload / request = %#v / %#v", structuredMap(t, edited), backend.lastEditText)
	}

	reacted, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.messages.react", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$event", "key": "👍"}})
	if err != nil {
		t.Fatalf("messages.react error = %v", err)
	}
	if structuredMap(t, reacted)["event_id"] != "$reaction" || backend.lastReact.Key != "👍" {
		t.Fatalf("unexpected react payload / request = %#v / %#v", structuredMap(t, reacted), backend.lastReact)
	}

	redacted, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "matrix.v1.messages.redact", Arguments: map[string]any{"room_id": "!joined:example.com", "event_id": "$reaction", "reason": "cleanup"}})
	if err != nil {
		t.Fatalf("messages.redact error = %v", err)
	}
	if structuredMap(t, redacted)["event_id"] != "$redaction" || backend.lastRedact.Reason != "cleanup" {
		t.Fatalf("unexpected redact payload / request = %#v / %#v", structuredMap(t, redacted), backend.lastRedact)
	}
}

func containsAll(body string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			return false
		}
	}
	return true
}
