package integration

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func TestEncryptedTimelineNeverReturnsCiphertextAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpe2eebot")
	peerUsername, peerPassword := registerUser(t, ctx, hs, "matrixmcpe2eepeer")

	session := newIntegrationSessionWithE2EE(t, ctx, hs, botUsername, botPassword, "default,rooms.join")
	botClient, err := hs.LoginClient(ctx, botUsername, botPassword)
	if err != nil {
		t.Fatalf("login bot user: %v", err)
	}
	peer := startCryptoClient(t, hs.HomeserverURL, peerUsername, peerPassword, filepath.Join(t.TempDir(), "peer-e2ee.db"))
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Fatalf("close crypto peer: %v", err)
		}
	})

	roomID := createEncryptedPrivateRoomAndInvite(t, peer.client, id.UserID(fmtUserID(botUsername)))
	joined := callToolMap(t, ctx, session, "matrix.v1.rooms.join", map[string]any{"room": roomID.String()})
	if joined["room_id"] != roomID.String() {
		t.Fatalf("rooms.join payload = %#v, want room_id %s", joined, roomID)
	}

	waitForJoinedMember(t, peer.client, roomID, id.UserID(fmtUserID(botUsername)), 30*time.Second)
	waitForJoinedMember(t, botClient, roomID, id.UserID(fmtUserID(botUsername)), 30*time.Second)
	peer.shareGroupSession(t, roomID, id.UserID(fmtUserID(peerUsername)), id.UserID(fmtUserID(botUsername)))

	if _, err := peer.client.SendText(ctx, roomID, "warmup"); err != nil {
		t.Fatalf("peer send warmup text: %v", err)
	}
	target, err := peer.client.SendText(ctx, roomID, "hello encrypted")
	if err != nil {
		t.Fatalf("peer send encrypted text: %v", err)
	}

	encryptionState := callToolMap(t, ctx, session, "matrix.v1.room.state.get", map[string]any{
		"room_id":    roomID.String(),
		"event_type": event.StateEncryption.Type,
	})
	if nestedMap(t, encryptionState, "event")["type"] != event.StateEncryption.Type {
		t.Fatalf("room.state.get encryption payload = %#v", encryptionState)
	}

	assertEncryptedTimelineStaysCiphertextFree(t, ctx, session, roomID.String(), target.EventID.String(), 15*time.Second)
}

func TestReplyToEncryptedEventAgainstTuwunel(t *testing.T) {
	ctx := integrationContext(t)
	hs := integrationHomeserver(t)
	botUsername, botPassword := registerUser(t, ctx, hs, "matrixmcpe2eereplybot")
	peerUsername, peerPassword := registerUser(t, ctx, hs, "matrixmcpe2eereplypeer")

	session := newIntegrationSessionWithE2EE(t, ctx, hs, botUsername, botPassword, "default,rooms.join")
	botClient, err := hs.LoginClient(ctx, botUsername, botPassword)
	if err != nil {
		t.Fatalf("login bot user: %v", err)
	}
	peer := startCryptoClient(t, hs.HomeserverURL, peerUsername, peerPassword, filepath.Join(t.TempDir(), "peer-reply-e2ee.db"))
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Fatalf("close crypto peer: %v", err)
		}
	})

	roomID := createEncryptedPrivateRoomAndInvite(t, peer.client, id.UserID(fmtUserID(botUsername)))
	joined := callToolMap(t, ctx, session, "matrix.v1.rooms.join", map[string]any{"room": roomID.String()})
	if joined["room_id"] != roomID.String() {
		t.Fatalf("rooms.join payload = %#v, want room_id %s", joined, roomID)
	}

	waitForJoinedMember(t, peer.client, roomID, id.UserID(fmtUserID(botUsername)), 30*time.Second)
	waitForJoinedMember(t, botClient, roomID, id.UserID(fmtUserID(botUsername)), 30*time.Second)
	peer.shareGroupSession(t, roomID, id.UserID(fmtUserID(peerUsername)), id.UserID(fmtUserID(botUsername)))

	if _, err := peer.client.SendText(ctx, roomID, "warmup"); err != nil {
		t.Fatalf("peer send warmup text: %v", err)
	}
	target, err := peer.client.SendText(ctx, roomID, "hello encrypted")
	if err != nil {
		t.Fatalf("peer send encrypted text: %v", err)
	}

	assertEncryptedTimelineStaysCiphertextFree(t, ctx, session, roomID.String(), target.EventID.String(), 15*time.Second)

	reply := callToolMap(t, ctx, session, "matrix.v1.messages.reply_text", map[string]any{
		"room_id":  roomID.String(),
		"event_id": target.EventID.String(),
		"body":     "reply from encrypted bot",
	})
	replyID, _ := reply["event_id"].(string)
	if replyID == "" {
		t.Fatalf("messages.reply_text payload = %#v", reply)
	}

	assertEventEventuallyExists(t, ctx, botClient, roomID, replyID, 15*time.Second)
}

type cryptoClient struct {
	client       *mautrix.Client
	cryptoHelper *cryptohelper.CryptoHelper
	cancel       context.CancelFunc
	done         chan error
}

func startCryptoClient(t *testing.T, homeserverURL, username, password, cryptoDBPath string) *cryptoClient {
	t.Helper()

	client, err := mautrix.NewClient(homeserverURL, "", "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.DefaultHTTPRetries = 3
	client.DefaultHTTPBackoff = 2 * time.Second

	helper, err := cryptohelper.NewCryptoHelper(client, []byte("0123456789abcdef0123456789abcdef"), cryptoDBPath)
	if err != nil {
		t.Fatalf("NewCryptoHelper() error = %v", err)
	}
	helper.LoginAs = &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password:                 password,
		InitialDeviceDisplayName: "matrix-mcp-integration",
	}
	if err := helper.Init(context.Background()); err != nil {
		_ = helper.Close()
		t.Fatalf("Init() error = %v", err)
	}
	if err := helper.Machine().ShareKeys(context.Background(), 50); err != nil {
		_ = helper.Close()
		t.Fatalf("ShareKeys() error = %v", err)
	}

	client.Crypto = helper
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- client.SyncWithContext(ctx)
	}()

	return &cryptoClient{
		client:       client,
		cryptoHelper: helper,
		cancel:       cancel,
		done:         done,
	}
}

func (c *cryptoClient) Close() error {
	if c == nil {
		return nil
	}

	c.cancel()
	select {
	case err := <-c.done:
		closeErr := c.cryptoHelper.Close()
		if err != nil && !errors.Is(err, context.Canceled) {
			return errors.Join(err, closeErr)
		}
		return closeErr
	case <-time.After(10 * time.Second):
		return errors.New("timed out stopping crypto client")
	}
}

func (c *cryptoClient) shareGroupSession(t *testing.T, roomID id.RoomID, users ...id.UserID) {
	t.Helper()
	if c == nil || c.cryptoHelper == nil {
		t.Fatal("shareGroupSession requires a crypto-enabled client")
	}
	if err := c.cryptoHelper.Machine().ShareGroupSession(context.Background(), roomID, users); err != nil {
		t.Fatalf("ShareGroupSession(%s) error = %v", roomID, err)
	}
}

func createEncryptedPrivateRoomAndInvite(t *testing.T, client *mautrix.Client, invitee id.UserID) id.RoomID {
	t.Helper()

	room, err := client.CreateRoom(context.Background(), &mautrix.ReqCreateRoom{
		Preset:   "private_chat",
		IsDirect: false,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	if _, err := client.SendStateEvent(
		context.Background(),
		room.RoomID,
		event.StateEncryption,
		"",
		&event.EncryptionEventContent{Algorithm: id.AlgorithmMegolmV1},
	); err != nil {
		t.Fatalf("SendStateEvent(m.room.encryption) error = %v", err)
	}
	waitForEncryptedRoom(t, client, room.RoomID, 10*time.Second)
	if _, err := client.InviteUser(context.Background(), room.RoomID, &mautrix.ReqInviteUser{
		UserID: invitee,
	}); err != nil {
		t.Fatalf("InviteUser() error = %v", err)
	}
	return room.RoomID
}

func waitForEncryptedRoom(t *testing.T, client *mautrix.Client, roomID id.RoomID, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		encrypted, err := client.StateStore.IsEncrypted(context.Background(), roomID)
		if err == nil && encrypted {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for encrypted state for room %s", roomID)
}

func waitForJoinedMember(t *testing.T, client *mautrix.Client, roomID id.RoomID, userID id.UserID, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		members, err := client.JoinedMembers(context.Background(), roomID)
		if err == nil {
			if _, ok := members.Joined[userID]; ok {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for join membership for %s in room %s", userID, roomID)
}

func assertEncryptedTimelineStaysCiphertextFree(t *testing.T, ctx context.Context, session *mcp.ClientSession, roomID string, eventID string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var sawTimeline bool
	for time.Now().Before(deadline) {
		timelineResult, err := callToolMapErr(ctx, session, "matrix.v1.timeline.messages.list", map[string]any{
			"room_id": roomID,
			"limit":   10,
		})
		if err == nil {
			sawTimeline = true
			events, _ := timelineResult["events"].([]any)
			if containsEncryptedEvent(events) {
				t.Fatalf("timeline.messages.list returned raw encrypted events: %#v", timelineResult)
			}
		}

		eventResult, err := callToolMapErr(ctx, session, "matrix.v1.timeline.event.get", map[string]any{
			"room_id":  roomID,
			"event_id": eventID,
		})
		if err == nil {
			summary := nestedMap(t, eventResult, "event")
			if summary["type"] == event.EventEncrypted.Type {
				t.Fatalf("timeline.event.get returned raw encrypted event: %#v", eventResult)
			}
		}

		time.Sleep(250 * time.Millisecond)
	}
	if !sawTimeline {
		t.Fatalf("timed out polling timeline.messages.list for room %s", roomID)
	}
}

func callToolMapErr(ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, error) {
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	if result.IsError || result.StructuredContent == nil {
		return nil, errors.New("tool returned error")
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func containsEncryptedEvent(events []any) bool {
	for _, raw := range events {
		eventMap, ok := raw.(map[string]any)
		if ok && eventMap["type"] == event.EventEncrypted.Type {
			return true
		}
	}
	return false
}

func assertEventEventuallyExists(t *testing.T, ctx context.Context, client *mautrix.Client, roomID id.RoomID, eventID string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := client.GetEvent(ctx, roomID, id.EventID(eventID)); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for event %s to exist in room %s", eventID, roomID)
}
