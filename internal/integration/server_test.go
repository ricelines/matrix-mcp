package integration

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/chat/matrix-mcp-go/internal/config"
	"github.com/ricelines/chat/matrix-mcp-go/internal/mcpserver"
	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
	"github.com/ricelines/chat/matrix-mcp-go/internal/testutil/tuwunel"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

func TestRegistrationAndDiscoveryAgainstTuwunel(t *testing.T) {
	if os.Getenv("MATRIX_MCP_GO_RUN_INTEGRATION") != "1" {
		t.Skip("set MATRIX_MCP_GO_RUN_INTEGRATION=1 to run dockerized Tuwunel integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	hs := tuwunel.Start(t, tuwunel.Options{RegistrationToken: "invite-only-token"})
	if err := hs.WaitUntilReady(ctx); err != nil {
		t.Fatal(err)
	}

	const botUsername = "matrixmcpgo"
	const botPassword = "matrix-mcp-go-secret"
	const peerUsername = "matrixmcppeer"
	const peerPassword = "matrix-mcp-peer-secret"
	const createdUsername = "matrixmcpcreated"
	const createdPassword = "matrix-mcp-created-secret"

	if err := hs.RegisterUser(ctx, botUsername, botPassword); err != nil {
		t.Fatal(err)
	}
	if err := hs.RegisterUser(ctx, peerUsername, peerPassword); err != nil {
		t.Fatal(err)
	}

	activeScopes, err := scopes.Parse("default,users.create")
	if err != nil {
		t.Fatal(err)
	}
	server, err := mcpserver.NewFromConfig(ctx, config.Config{
		ListenAddr:    ":0",
		HomeserverURL: hs.HomeserverURL,
		Username:      botUsername,
		Password:      botPassword,
		Scopes:        activeScopes,
	})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "integration-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	assertResourceContains(t, ctx, session, "matrix://modules", "matrix://module/timeline")
	assertResourceContains(t, ctx, session, "matrix://module/users", "matrix.v1.users.create")
	assertResourceContains(t, ctx, session, "matrix://tool/matrix.v1.users.create", "registration_token")
	assertResourceContains(t, ctx, session, "matrix://scopes", "timeline.read")

	identity := callToolMap(t, ctx, session, "matrix.v1.client.identity.get", nil)
	if identity["user_id"] == "" {
		t.Fatalf("identity payload missing user_id: %#v", identity)
	}
	status := callToolMap(t, ctx, session, "matrix.v1.client.status.get", nil)
	if status["is_active"] != true {
		t.Fatalf("status payload = %#v", status)
	}
	versions := callToolMap(t, ctx, session, "matrix.v1.server.versions.get", nil)
	if len(versions["versions"].([]any)) == 0 {
		t.Fatalf("versions payload = %#v", versions)
	}
	capabilities := callToolMap(t, ctx, session, "matrix.v1.server.capabilities.get", nil)
	if capabilities["capabilities"] == nil {
		t.Fatalf("capabilities payload = %#v", capabilities)
	}

	available := callToolMap(t, ctx, session, "matrix.v1.users.register_available", map[string]any{"username": createdUsername})
	if available["available"] != true {
		t.Fatalf("register_available payload = %#v", available)
	}

	created := callToolMap(t, ctx, session, "matrix.v1.users.create", map[string]any{
		"username":                    createdUsername,
		"password":                    createdPassword,
		"registration_token":          hs.RegistrationToken,
		"initial_device_display_name": "matrix-mcp-go integration",
	})
	if created["user_id"] == "" {
		t.Fatalf("users.create payload = %#v", created)
	}
	if _, err := hs.LoginClient(ctx, createdUsername, createdPassword); err != nil {
		t.Fatalf("login created user: %v", err)
	}

	availableAfter := callToolMap(t, ctx, session, "matrix.v1.users.register_available", map[string]any{"username": createdUsername})
	if availableAfter["available"] != false {
		t.Fatalf("register_available after create payload = %#v", availableAfter)
	}
}

func TestConversationReadWriteAgainstTuwunel(t *testing.T) {
	if os.Getenv("MATRIX_MCP_GO_RUN_INTEGRATION") != "1" {
		t.Skip("set MATRIX_MCP_GO_RUN_INTEGRATION=1 to run dockerized Tuwunel integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	hs := tuwunel.Start(t, tuwunel.Options{RegistrationToken: "invite-only-token"})
	if err := hs.WaitUntilReady(ctx); err != nil {
		t.Fatal(err)
	}

	const botUsername = "matrixmcpgo2"
	const botPassword = "matrix-mcp-go-secret-2"
	const peerUsername = "matrixmcppeer2"
	const peerPassword = "matrix-mcp-peer-secret-2"
	const inviteeUsername = "matrixmcpinvitee"
	const inviteePassword = "matrix-mcp-invitee-secret"

	if err := hs.RegisterUser(ctx, botUsername, botPassword); err != nil {
		t.Fatal(err)
	}
	if err := hs.RegisterUser(ctx, peerUsername, peerPassword); err != nil {
		t.Fatal(err)
	}

	activeScopes, err := scopes.Parse("default,users.create,rooms.create,rooms.join,messages.send,messages.reply,messages.edit,messages.react,messages.redact")
	if err != nil {
		t.Fatal(err)
	}
	server, err := mcpserver.NewFromConfig(ctx, config.Config{
		ListenAddr:    ":0",
		HomeserverURL: hs.HomeserverURL,
		Username:      botUsername,
		Password:      botPassword,
		Scopes:        activeScopes,
	})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "integration-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	invitee := callToolMap(t, ctx, session, "matrix.v1.users.create", map[string]any{
		"username":           inviteeUsername,
		"password":           inviteePassword,
		"registration_token": hs.RegistrationToken,
	})
	inviteeUserID := invitee["user_id"].(string)
	inviteeClient, err := hs.LoginClient(ctx, inviteeUsername, inviteePassword)
	if err != nil {
		t.Fatalf("login invitee user: %v", err)
	}

	peerClient, err := hs.LoginClient(ctx, peerUsername, peerPassword)
	if err != nil {
		t.Fatalf("login peer user: %v", err)
	}
	lobby, err := peerClient.CreateRoom(ctx, &mautrix.ReqCreateRoom{Name: "Lobby", Visibility: "public", Preset: "public_chat"})
	if err != nil {
		t.Fatalf("peer create lobby: %v", err)
	}

	joined := callToolMap(t, ctx, session, "matrix.v1.rooms.join", map[string]any{"room": lobby.RoomID.String()})
	if joined["room_id"] != lobby.RoomID.String() {
		t.Fatalf("rooms.join payload = %#v", joined)
	}

	rooms := callToolMap(t, ctx, session, "matrix.v1.rooms.list", nil)
	roomsList := rooms["rooms"].([]any)
	if len(roomsList) == 0 {
		t.Fatalf("rooms.list payload = %#v", rooms)
	}

	roomSummary := callToolMap(t, ctx, session, "matrix.v1.rooms.get", map[string]any{"room_id": lobby.RoomID.String()})
	if roomSummary["room"].(map[string]any)["room_id"] != lobby.RoomID.String() {
		t.Fatalf("rooms.get payload = %#v", roomSummary)
	}

	roomPreview := callToolMap(t, ctx, session, "matrix.v1.rooms.preview", map[string]any{"room": lobby.RoomID.String()})
	if roomPreview["room"].(map[string]any)["room_id"] != lobby.RoomID.String() {
		t.Fatalf("rooms.preview payload = %#v", roomPreview)
	}

	privateRoom := callToolMap(t, ctx, session, "matrix.v1.rooms.create", map[string]any{
		"name":   "Go MCP Room",
		"topic":  "integration",
		"invite": []string{fmtUserID(peerUsername), inviteeUserID},
	})
	roomID := privateRoom["room_id"].(string)
	if roomID == "" {
		t.Fatalf("rooms.create payload = %#v", privateRoom)
	}

	if _, err := peerClient.JoinRoom(ctx, roomID, nil); err != nil {
		t.Fatalf("peer join room: %v", err)
	}
	if _, err := inviteeClient.JoinRoom(ctx, roomID, nil); err != nil {
		t.Fatalf("invitee join room: %v", err)
	}

	members := callToolMap(t, ctx, session, "matrix.v1.room.members.list", map[string]any{"room_id": roomID})
	memberList := members["members"].([]any)
	if len(memberList) < 3 {
		t.Fatalf("room.members.list payload = %#v", members)
	}

	member := callToolMap(t, ctx, session, "matrix.v1.room.members.get", map[string]any{"room_id": roomID, "user_id": fmtUserID(peerUsername)})
	if member["member"].(map[string]any)["user_id"] != fmtUserID(peerUsername) {
		t.Fatalf("room.members.get payload = %#v", member)
	}

	stateGet := callToolMap(t, ctx, session, "matrix.v1.room.state.get", map[string]any{"room_id": roomID, "event_type": "m.room.create"})
	if stateGet["event"].(map[string]any)["type"] != "m.room.create" {
		t.Fatalf("room.state.get payload = %#v", stateGet)
	}

	stateList := callToolMap(t, ctx, session, "matrix.v1.room.state.list", map[string]any{"room_id": roomID})
	if len(stateList["events"].([]any)) == 0 {
		t.Fatalf("room.state.list payload = %#v", stateList)
	}

	seed, err := peerClient.SendText(ctx, id.RoomID(roomID), "hello from peer")
	if err != nil {
		t.Fatalf("peer send text: %v", err)
	}
	seedID := seed.EventID.String()

	eventGet := callToolMap(t, ctx, session, "matrix.v1.timeline.event.get", map[string]any{"room_id": roomID, "event_id": seedID})
	if eventGet["event"].(map[string]any)["event_id"] != seedID {
		t.Fatalf("timeline.event.get payload = %#v", eventGet)
	}

	reply := callToolMap(t, ctx, session, "matrix.v1.messages.reply_text", map[string]any{"room_id": roomID, "event_id": seedID, "body": "reply from bot"})
	replyID := reply["event_id"].(string)
	if replyID == "" {
		t.Fatalf("messages.reply_text payload = %#v", reply)
	}

	sent := callToolMap(t, ctx, session, "matrix.v1.messages.send_text", map[string]any{"room_id": roomID, "body": "plain message"})
	sentID := sent["event_id"].(string)
	if sentID == "" {
		t.Fatalf("messages.send_text payload = %#v", sent)
	}

	edited := callToolMap(t, ctx, session, "matrix.v1.messages.edit_text", map[string]any{"room_id": roomID, "event_id": sentID, "body": "edited message"})
	editID := edited["event_id"].(string)
	if editID == "" {
		t.Fatalf("messages.edit_text payload = %#v", edited)
	}

	reacted := callToolMap(t, ctx, session, "matrix.v1.messages.react", map[string]any{"room_id": roomID, "event_id": seedID, "key": "👍"})
	reactionID := reacted["event_id"].(string)
	if reactionID == "" {
		t.Fatalf("messages.react payload = %#v", reacted)
	}

	messages := callToolMap(t, ctx, session, "matrix.v1.timeline.messages.list", map[string]any{"room_id": roomID, "limit": 20})
	if len(messages["events"].([]any)) == 0 {
		t.Fatalf("timeline.messages.list payload = %#v", messages)
	}

	contextResult := callToolMap(t, ctx, session, "matrix.v1.timeline.event.context.get", map[string]any{"room_id": roomID, "event_id": replyID, "limit": 5})
	if contextResult["event"].(map[string]any)["event_id"] != replyID {
		t.Fatalf("timeline.event.context.get payload = %#v", contextResult)
	}

	annotationRelations := callToolMap(t, ctx, session, "matrix.v1.timeline.relations.list", map[string]any{"room_id": roomID, "event_id": seedID, "relation_type": "m.annotation", "event_type": "m.reaction"})
	annotationEvents := annotationRelations["events"].([]any)
	if !containsEvent(annotationEvents, reactionID) {
		t.Fatalf("timeline.relations.list annotation payload = %#v", annotationRelations)
	}

	replaceRelations := callToolMap(t, ctx, session, "matrix.v1.timeline.relations.list", map[string]any{"room_id": roomID, "event_id": sentID, "relation_type": "m.replace", "event_type": "m.room.message"})
	replaceEvents := replaceRelations["events"].([]any)
	if !containsEvent(replaceEvents, editID) {
		t.Fatalf("timeline.relations.list replace payload = %#v", replaceRelations)
	}

	redacted := callToolMap(t, ctx, session, "matrix.v1.messages.redact", map[string]any{"room_id": roomID, "event_id": reactionID, "reason": "cleanup"})
	redactionID := redacted["event_id"].(string)
	if redactionID == "" {
		t.Fatalf("messages.redact payload = %#v", redacted)
	}

	botClient, err := hs.LoginClient(ctx, botUsername, botPassword)
	if err != nil {
		t.Fatalf("login bot user: %v", err)
	}
	if _, err := botClient.GetEvent(ctx, id.RoomID(roomID), id.EventID(redactionID)); err != nil {
		t.Fatalf("GetEvent(redaction) error = %v", err)
	}
}

func assertResourceContains(t *testing.T, ctx context.Context, session *mcp.ClientSession, uri string, needle string) {
	t.Helper()
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource(%s) error = %v", uri, err)
	}
	if !strings.Contains(result.Contents[0].Text, needle) {
		t.Fatalf("resource %s did not contain %q:\n%s", uri, needle, result.Contents[0].Text)
	}
}

func callToolMap(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", name, err)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	return decoded
}

func containsEvent(events []any, eventID string) bool {
	for _, raw := range events {
		eventMap, ok := raw.(map[string]any)
		if ok && eventMap["event_id"] == eventID {
			return true
		}
	}
	return false
}

func fmtUserID(localpart string) string {
	return "@" + localpart + ":localhost"
}
