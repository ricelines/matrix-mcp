package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"maunium.net/go/mautrix/crypto/attachment"
)

func rawResponse(req *http.Request, status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
}

func mediaTestService(t *testing.T, content map[string]any, eventType string, blob []byte) *Service {
	t.Helper()
	return newClientTestService(t, func(r *http.Request) *http.Response {
		switch r.URL.Path {
		case "/_matrix/client/v3/rooms/!room:example.com/event/$voice":
			return jsonResponse(t, r, http.StatusOK, map[string]any{
				"type":             eventType,
				"event_id":         "$voice",
				"room_id":          "!room:example.com",
				"sender":           "@whatsapp_1:example.com",
				"origin_server_ts": 1,
				"content":          content,
			})
		case "/_matrix/client/v1/media/download/example.com/abc":
			return rawResponse(r, http.StatusOK, "application/octet-stream", blob)
		default:
			return jsonResponse(t, r, http.StatusNotFound, map[string]any{"errcode": "M_NOT_FOUND"})
		}
	})
}

func TestDownloadEventMediaDecryptsEncryptedAttachment(t *testing.T) {
	plaintext := []byte("OggS fake opus voice note")
	file := attachment.NewEncryptedFile()
	ciphertext := bytes.Clone(plaintext)
	file.EncryptInPlace(ciphertext)

	var fileJSON map[string]any
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("marshal encrypted file: %v", err)
	}
	if err := json.Unmarshal(raw, &fileJSON); err != nil {
		t.Fatalf("unmarshal encrypted file: %v", err)
	}
	fileJSON["url"] = "mxc://example.com/abc"

	svc := mediaTestService(t, map[string]any{
		"msgtype": "m.audio",
		"body":    "Voice message.ogg",
		"file":    fileJSON,
		"info":    map[string]any{"mimetype": "audio/ogg", "size": len(plaintext), "duration": 31000},
	}, "m.room.message", ciphertext)

	info, data, err := svc.DownloadEventMedia(context.Background(), "!room:example.com", "$voice")
	if err != nil {
		t.Fatalf("DownloadEventMedia() error = %v", err)
	}
	if !bytes.Equal(data, plaintext) {
		t.Fatalf("decrypted data = %q, want %q", data, plaintext)
	}
	want := MediaInfo{
		RoomID: "!room:example.com", EventID: "$voice", Sender: "@whatsapp_1:example.com",
		MsgType: "m.audio", FileName: "Voice message.ogg", MIMEType: "audio/ogg",
		Size: len(plaintext), DurationMS: 31000, Encrypted: true,
	}
	if info != want {
		t.Fatalf("info = %#v, want %#v", info, want)
	}

	// A tampered download must fail the hash check instead of returning garbage.
	tampered := bytes.Clone(ciphertext)
	tampered[0] ^= 0xff
	svc = mediaTestService(t, map[string]any{"msgtype": "m.audio", "body": "v", "file": fileJSON}, "m.room.message", tampered)
	if _, _, err := svc.DownloadEventMedia(context.Background(), "!room:example.com", "$voice"); !errors.Is(err, attachment.ErrHashMismatch) {
		t.Fatalf("DownloadEventMedia(tampered) error = %v, want hash mismatch", err)
	}
}

func TestDownloadEventMediaPlainURL(t *testing.T) {
	blob := []byte("\x89PNG fake")
	svc := mediaTestService(t, map[string]any{
		"msgtype":  "m.image",
		"body":     "look at this",
		"filename": "photo.png",
		"url":      "mxc://example.com/abc",
		"info":     map[string]any{"mimetype": "image/png", "w": 640, "h": 480},
	}, "m.room.message", blob)

	info, data, err := svc.DownloadEventMedia(context.Background(), "!room:example.com", "$voice")
	if err != nil {
		t.Fatalf("DownloadEventMedia() error = %v", err)
	}
	if !bytes.Equal(data, blob) || info.Encrypted || info.FileName != "photo.png" || info.Width != 640 || info.Height != 480 {
		t.Fatalf("got info %#v data %q", info, data)
	}
}

func TestResolveEventMediaRejectsEventsWithoutMedia(t *testing.T) {
	svc := mediaTestService(t, map[string]any{"msgtype": "m.text", "body": "hello"}, "m.room.message", nil)
	if _, err := svc.ResolveEventMedia(context.Background(), "!room:example.com", "$voice"); !errors.Is(err, ErrNoMedia) {
		t.Fatalf("ResolveEventMedia(text) error = %v, want ErrNoMedia", err)
	}
	svc = mediaTestService(t, map[string]any{"name": "Room"}, "m.room.name", nil)
	if _, err := svc.ResolveEventMedia(context.Background(), "!room:example.com", "$voice"); !errors.Is(err, ErrNoMedia) {
		t.Fatalf("ResolveEventMedia(state) error = %v, want ErrNoMedia", err)
	}
}
