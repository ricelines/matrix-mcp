package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	matrixclient "github.com/ricelines/chat/matrix-mcp-go/internal/matrix"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

func TestRegistrationAndDiscoveryAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpbot")
	_, _ = registerUser(t, ctx, hs, "matrixmcppeer")
	createdUsername, createdPassword := uniqueCredentials("matrixmcpcreated")
	session := newSession(t, ctx, hs, botUsername, botPassword, "default,users.create")

	assertResourceContains(t, ctx, session, "matrix://modules", "matrix://module/timeline")
	assertResourceContains(t, ctx, session, "matrix://module/users", "matrix.v1.users.create")
	assertResourceNotContains(t, ctx, session, "matrix://tool/matrix.v1.users.create", "registration_token")
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
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpbot")
	peerUsername, _, peerClient := registerAndLoginUser(t, ctx, hs, "matrixmcppeer")
	inviteeUsername, inviteePassword := uniqueCredentials("matrixmcpinvitee")
	session := newSession(t, ctx, hs, botUsername, botPassword, "default,users.create,rooms.create,rooms.join,messages.send,messages.reply,messages.edit,messages.react,messages.redact")

	invitee := callToolMap(t, ctx, session, "matrix.v1.users.create", map[string]any{
		"username": inviteeUsername,
		"password": inviteePassword,
	})
	inviteeUserID := invitee["user_id"].(string)
	inviteeClient, err := hs.LoginClient(ctx, inviteeUsername, inviteePassword)
	if err != nil {
		t.Fatalf("login invitee user: %v", err)
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
	replyEvent := callToolMap(t, ctx, session, "matrix.v1.timeline.event.get", map[string]any{"room_id": roomID, "event_id": replyID})
	replyContent := nestedMap(t, nestedMap(t, replyEvent, "event"), "content")
	replyRelation := nestedMap(t, replyContent, "m.relates_to")
	if nestedMap(t, replyRelation, "m.in_reply_to")["event_id"] != seedID {
		t.Fatalf("reply event did not point at original event: %#v", replyEvent)
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
	editEvent := callToolMap(t, ctx, session, "matrix.v1.timeline.event.get", map[string]any{"room_id": roomID, "event_id": editID})
	editContent := nestedMap(t, nestedMap(t, editEvent, "event"), "content")
	editRelation := nestedMap(t, editContent, "m.relates_to")
	if editRelation["rel_type"] != "m.replace" || editRelation["event_id"] != sentID {
		t.Fatalf("edit event relation payload = %#v", editEvent)
	}
	if nestedMap(t, editContent, "m.new_content")["body"] != "edited message" {
		t.Fatalf("edit event new content payload = %#v", editEvent)
	}

	reacted := callToolMap(t, ctx, session, "matrix.v1.messages.react", map[string]any{"room_id": roomID, "event_id": seedID, "key": "👍"})
	reactionID := reacted["event_id"].(string)
	if reactionID == "" {
		t.Fatalf("messages.react payload = %#v", reacted)
	}
	reactionEvent := callToolMap(t, ctx, session, "matrix.v1.timeline.event.get", map[string]any{"room_id": roomID, "event_id": reactionID})
	reactionRelation := nestedMap(t, nestedMap(t, nestedMap(t, reactionEvent, "event"), "content"), "m.relates_to")
	if reactionRelation["rel_type"] != "m.annotation" || reactionRelation["event_id"] != seedID || reactionRelation["key"] != "👍" {
		t.Fatalf("reaction event payload = %#v", reactionEvent)
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
	redactionEvent := callToolMap(t, ctx, session, "matrix.v1.timeline.event.get", map[string]any{"room_id": roomID, "event_id": redactionID})
	if nestedMap(t, redactionEvent, "event")["redacts"] != reactionID {
		t.Fatalf("redaction event payload = %#v", redactionEvent)
	}

	botClient, err := hs.LoginClient(ctx, botUsername, botPassword)
	if err != nil {
		t.Fatalf("login bot user: %v", err)
	}
	if _, err := botClient.GetEvent(ctx, id.RoomID(roomID), id.EventID(redactionID)); err != nil {
		t.Fatalf("GetEvent(redaction) error = %v", err)
	}
}

func TestPublicRoomCreationAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpbot")
	_, _, peerClient := registerAndLoginUser(t, ctx, hs, "matrixmcppeer")
	session := newSession(t, ctx, hs, botUsername, botPassword, "default,rooms.create")

	publicRoom := callToolMap(t, ctx, session, "matrix.v1.rooms.create", map[string]any{
		"name":      "Public Integration Room",
		"topic":     "shared-homeserver coverage",
		"is_public": true,
	})
	roomID := publicRoom["room_id"].(string)
	if roomID == "" {
		t.Fatalf("rooms.create public payload = %#v", publicRoom)
	}

	if _, err := peerClient.JoinRoom(ctx, roomID, nil); err != nil {
		t.Fatalf("peer join tool-created public room: %v", err)
	}

	summary := callToolMap(t, ctx, session, "matrix.v1.rooms.get", map[string]any{"room_id": roomID})
	room := nestedMap(t, summary, "room")
	if room["room_id"] != roomID || room["topic"] != "shared-homeserver coverage" {
		t.Fatalf("rooms.get public room payload = %#v", summary)
	}

	joinRules := callToolMap(t, ctx, session, "matrix.v1.room.state.get", map[string]any{
		"room_id":    roomID,
		"event_type": "m.room.join_rules",
	})
	joinRuleEvent := nestedMap(t, joinRules, "event")
	if joinRuleEvent["state_key"] != "" || nestedMap(t, joinRuleEvent, "content")["join_rule"] != "public" {
		t.Fatalf("room.state.get join rules payload = %#v", joinRules)
	}
}

func TestRoomAliasAndDirectoryAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpbot")
	session := newSession(t, ctx, hs, botUsername, botPassword, "default,rooms.alias.read,rooms.alias.write,rooms.directory.read,rooms.directory.write")

	botClient, err := hs.LoginClient(ctx, botUsername, botPassword)
	if err != nil {
		t.Fatalf("login bot user: %v", err)
	}

	room, err := botClient.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:       "Alias Directory Coverage",
		Visibility: "private",
		Preset:     "private_chat",
	})
	if err != nil {
		t.Fatalf("bot create room: %v", err)
	}
	roomID := room.RoomID.String()
	roomAlias := "#" + nextUsername("matrixmcpalias") + ":localhost"

	createdAlias := callToolMap(t, ctx, session, "matrix.v1.rooms.alias.create", map[string]any{
		"room_alias": roomAlias,
		"room_id":    roomID,
	})
	if createdAlias["room_alias"] != roomAlias || createdAlias["room_id"] != roomID {
		t.Fatalf("rooms.alias.create payload = %#v", createdAlias)
	}

	resolvedAlias := callToolMap(t, ctx, session, "matrix.v1.rooms.alias.get", map[string]any{"room_alias": roomAlias})
	if resolvedAlias["room_id"] != roomID {
		t.Fatalf("rooms.alias.get payload = %#v", resolvedAlias)
	}
	if _, err := botClient.ResolveAlias(ctx, id.RoomAlias(roomAlias)); err != nil {
		t.Fatalf("ResolveAlias(created alias) error = %v", err)
	}

	published := callToolMap(t, ctx, session, "matrix.v1.rooms.directory.publish", map[string]any{"room_id": roomID})
	if published["visibility"] != matrixclient.RoomDirectoryVisibilityPublic {
		t.Fatalf("rooms.directory.publish payload = %#v", published)
	}
	if got := roomDirectoryVisibility(t, ctx, botClient, roomID); got != matrixclient.RoomDirectoryVisibilityPublic {
		t.Fatalf("room directory visibility after publish = %q", got)
	}

	directory := callToolMap(t, ctx, session, "matrix.v1.rooms.directory.get", map[string]any{"room_id": roomID})
	if directory["visibility"] != matrixclient.RoomDirectoryVisibilityPublic {
		t.Fatalf("rooms.directory.get payload after publish = %#v", directory)
	}

	unpublished := callToolMap(t, ctx, session, "matrix.v1.rooms.directory.unpublish", map[string]any{"room_id": roomID})
	if unpublished["visibility"] != matrixclient.RoomDirectoryVisibilityPrivate {
		t.Fatalf("rooms.directory.unpublish payload = %#v", unpublished)
	}
	if got := roomDirectoryVisibility(t, ctx, botClient, roomID); got != matrixclient.RoomDirectoryVisibilityPrivate {
		t.Fatalf("room directory visibility after unpublish = %q", got)
	}

	deletedAlias := callToolMap(t, ctx, session, "matrix.v1.rooms.alias.delete", map[string]any{"room_alias": roomAlias})
	if deletedAlias["room_alias"] != roomAlias {
		t.Fatalf("rooms.alias.delete payload = %#v", deletedAlias)
	}
	if _, err := botClient.ResolveAlias(ctx, id.RoomAlias(roomAlias)); err == nil {
		t.Fatal("ResolveAlias(deleted alias) unexpectedly succeeded")
	}
}

func TestTimelinePaginationAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpbot")
	peerUsername, _, peerClient := registerAndLoginUser(t, ctx, hs, "matrixmcppeer")
	session := newSession(t, ctx, hs, botUsername, botPassword, "default,rooms.create")

	privateRoom := callToolMap(t, ctx, session, "matrix.v1.rooms.create", map[string]any{
		"name":   "Pagination Room",
		"invite": []string{fmtUserID(peerUsername)},
	})
	roomID := privateRoom["room_id"].(string)
	if roomID == "" {
		t.Fatalf("rooms.create pagination payload = %#v", privateRoom)
	}

	if _, err := peerClient.JoinRoom(ctx, roomID, nil); err != nil {
		t.Fatalf("peer join pagination room: %v", err)
	}

	first, err := peerClient.SendText(ctx, id.RoomID(roomID), "one")
	if err != nil {
		t.Fatalf("peer send first text: %v", err)
	}
	second, err := peerClient.SendText(ctx, id.RoomID(roomID), "two")
	if err != nil {
		t.Fatalf("peer send second text: %v", err)
	}
	third, err := peerClient.SendText(ctx, id.RoomID(roomID), "three")
	if err != nil {
		t.Fatalf("peer send third text: %v", err)
	}

	firstPage := callToolMap(t, ctx, session, "matrix.v1.timeline.messages.list", map[string]any{
		"room_id":   roomID,
		"direction": "b",
		"limit":     2,
	})
	firstPageIDs := eventIDs(firstPage["events"].([]any))
	if len(firstPageIDs) != 2 || !containsEventID(firstPageIDs, second.EventID.String()) || !containsEventID(firstPageIDs, third.EventID.String()) || containsEventID(firstPageIDs, first.EventID.String()) {
		t.Fatalf("first pagination page payload = %#v", firstPage)
	}

	endToken, _ := firstPage["end"].(string)
	if endToken == "" {
		t.Fatalf("first pagination page missing end token: %#v", firstPage)
	}

	secondPage := callToolMap(t, ctx, session, "matrix.v1.timeline.messages.list", map[string]any{
		"room_id":   roomID,
		"from":      endToken,
		"direction": "b",
		"limit":     10,
	})
	if !containsEventID(eventIDs(secondPage["events"].([]any)), first.EventID.String()) {
		t.Fatalf("second pagination page payload = %#v", secondPage)
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

func assertResourceNotContains(t *testing.T, ctx context.Context, session *mcp.ClientSession, uri string, needle string) {
	t.Helper()
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource(%s) error = %v", uri, err)
	}
	if strings.Contains(result.Contents[0].Text, needle) {
		t.Fatalf("resource %s unexpectedly contained %q:\n%s", uri, needle, result.Contents[0].Text)
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

func nestedMap(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := value[key]
	if !ok {
		t.Fatalf("missing key %q in %#v", key, value)
	}
	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("value for %q is %T, want map[string]any", key, raw)
	}
	return result
}

func eventIDs(events []any) []string {
	ids := make([]string, 0, len(events))
	for _, raw := range events {
		eventMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		eventID, ok := eventMap["event_id"].(string)
		if ok {
			ids = append(ids, eventID)
		}
	}
	return ids
}

func containsEventID(eventIDs []string, eventID string) bool {
	for _, candidate := range eventIDs {
		if candidate == eventID {
			return true
		}
	}
	return false
}

func roomDirectoryVisibility(t *testing.T, ctx context.Context, client *mautrix.Client, roomID string) string {
	t.Helper()

	var response struct {
		Visibility string `json:"visibility"`
	}
	urlPath := client.BuildClientURL("v3", "directory", "list", "room", id.RoomID(roomID))
	if _, err := client.MakeRequest(ctx, http.MethodGet, urlPath, nil, &response); err != nil {
		t.Fatalf("get room directory visibility: %v", err)
	}
	return response.Visibility
}
