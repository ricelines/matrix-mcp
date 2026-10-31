package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestDetectMIMEType(t *testing.T) {
	cases := map[string]string{
		"qr.PNG":       "image/png",
		"Профиль.ovpn": "application/x-openvpn-profile",
		"wg0.conf":     "text/plain",
		"report.pdf":   "application/pdf",
	}
	for name, want := range cases {
		if got := DetectMIMEType(name, nil); got != want {
			t.Errorf("DetectMIMEType(%q) = %q, want %q", name, got, want)
		}
	}
	if got := DetectMIMEType("noext", tinyPNG(t)); got != "image/png" {
		t.Errorf("sniffed type = %q, want image/png", got)
	}
}

func TestBuildFileContentImageWithCaption(t *testing.T) {
	data := tinyPNG(t)
	content, payload := buildFileContent(SendFileRequest{FileName: "mother.png", Data: data, Caption: "Подпись"}, false)
	if content.MsgType != event.MsgImage || content.Body != "Подпись" || content.FileName != "mother.png" {
		t.Fatalf("unexpected content %#v", content)
	}
	if content.Info.MimeType != "image/png" || content.Info.Size != len(data) || content.Info.Width != 3 || content.Info.Height != 2 {
		t.Fatalf("unexpected info %#v", content.Info)
	}
	if !bytes.Equal(payload, data) || content.File != nil {
		t.Fatal("plain room must upload the file as is")
	}
}

func TestBuildFileContentAsFileWithoutCaption(t *testing.T) {
	content, _ := buildFileContent(SendFileRequest{FileName: "qr.png", Data: tinyPNG(t), AsFile: true}, false)
	if content.MsgType != event.MsgFile || content.Body != "qr.png" || content.FileName != "" {
		t.Fatalf("unexpected content %#v", content)
	}
}

func TestBuildFileContentEncryptsInEncryptedRooms(t *testing.T) {
	data := []byte("client\nremote vpn.example.com 1194 udp\n")
	content, payload := buildFileContent(SendFileRequest{FileName: "s.ovpn", Data: data}, true)
	if content.File == nil || bytes.Equal(payload, data) {
		t.Fatal("encrypted room must upload ciphertext and carry the key")
	}
	setFileURI(content, id.ContentURI{Homeserver: "example.com", FileID: "abc"})
	if content.URL != "" || content.File.URL != "mxc://example.com/abc" {
		t.Fatalf("URI went to the wrong field: url=%q file.url=%q", content.URL, content.File.URL)
	}
	// Decrypt the way a recipient does: from the event JSON, not the sender's
	// in-memory struct (which caches keys without the hash).
	raw, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var received event.MessageEventContent
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	decrypted := bytes.Clone(payload)
	if err := received.File.DecryptInPlace(decrypted); err != nil || !bytes.Equal(decrypted, data) {
		t.Fatalf("ciphertext does not decrypt back: %v", err)
	}
}

func TestSendFileUploadsAndPosts(t *testing.T) {
	data := tinyPNG(t)
	var uploaded []byte
	var uploadType string
	var sent map[string]any
	svc := newClientTestService(t, func(r *http.Request) *http.Response {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/_matrix/media/v3/upload":
			uploaded, _ = io.ReadAll(r.Body)
			uploadType = r.Header.Get("Content-Type")
			return jsonResponse(t, r, http.StatusOK, map[string]any{"content_uri": "mxc://example.com/up1"})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.EscapedPath(), "/_matrix/client/v3/rooms/%21room:example.com/send/m.room.message/"):
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &sent)
			return jsonResponse(t, r, http.StatusOK, map[string]any{"event_id": "$file"})
		default:
			return jsonResponse(t, r, http.StatusNotFound, map[string]any{"errcode": "M_NOT_FOUND"})
		}
	})

	res, err := svc.SendFile(context.Background(), SendFileRequest{RoomID: "!room:example.com", FileName: "qr.png", Data: data, Caption: "Подпись"})
	if err != nil {
		t.Fatalf("SendFile() error = %v", err)
	}
	if res.EventID != "$file" || !bytes.Equal(uploaded, data) || uploadType != "image/png" {
		t.Fatalf("event=%q uploaded=%d bytes type=%q", res.EventID, len(uploaded), uploadType)
	}
	if sent["msgtype"] != "m.image" || sent["url"] != "mxc://example.com/up1" || sent["body"] != "Подпись" || sent["filename"] != "qr.png" {
		t.Fatalf("unexpected sent content %#v", sent)
	}
}
