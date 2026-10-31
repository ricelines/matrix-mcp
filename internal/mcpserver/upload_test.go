package mcpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s error = %v", name, err)
	}
	return res
}

func TestUploadSlotThenSendFile(t *testing.T) {
	backend := &fakeMatrix{}
	server := NewWithOptions(backend, scopes.Default(), Options{PublicURL: "https://mcp.example.com"})
	session := connectTestSession(t, server)
	routes := AccessLog(server.Routes("s3cret"))
	logs := captureLog(t)

	slot := structuredMap(t, callTool(t, session, "matrix.v1.media.upload_slot", map[string]any{}))
	uploadURL, _ := slot["upload_url"].(string)
	uploadID, _ := slot["upload_id"].(string)
	if !strings.HasPrefix(uploadURL, "https://mcp.example.com/upload/"+uploadID+"/") || uploadID == "" {
		t.Fatalf("unexpected slot %#v", slot)
	}

	// What `curl -T "4 Фото.png" <url>` sends: PUT with the name appended.
	path := strings.TrimPrefix(uploadURL, "https://mcp.example.com") + "4%20%D0%A4%D0%BE%D1%82%D0%BE.png"
	put := func() int {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader("\x89PNG fake")))
		return rec.Code
	}
	if code := put(); code != http.StatusCreated {
		t.Fatalf("PUT status = %d", code)
	}
	if code := put(); code != http.StatusConflict {
		t.Fatalf("second PUT status = %d, want 409", code)
	}
	if strings.Contains(logs.String(), uploadID) || strings.Contains(logs.String(), "%D0%A4") {
		t.Fatalf("token or file name leaked into the log: %s", logs.String())
	}

	backend.sendFileErr = errors.New("homeserver down")
	failed := callTool(t, session, "matrix.v1.messages.send_file", map[string]any{"room_id": "!room:example.com", "upload_id": uploadID, "caption": "Подпись"})
	if !failed.IsError {
		t.Fatal("send_file must report the backend error")
	}
	backend.sendFileErr = nil
	sent := callTool(t, session, "matrix.v1.messages.send_file", map[string]any{"room_id": "!room:example.com", "upload_id": uploadID, "caption": "Подпись", "as_file": true})
	if sent.IsError || structuredMap(t, sent)["event_id"] != "$file" {
		t.Fatalf("send_file after retry failed: %#v", sent.Content)
	}
	got := backend.lastSendFile
	if got.FileName != "4 Фото.png" || got.MIMEType != "image/png" || string(got.Data) != "\x89PNG fake" || got.Caption != "Подпись" || !got.AsFile {
		t.Fatalf("unexpected send request %#v", got)
	}

	// The slot is consumed by a successful send.
	again := callTool(t, session, "matrix.v1.messages.send_file", map[string]any{"room_id": "!room:example.com", "upload_id": uploadID})
	if !again.IsError {
		t.Fatal("a consumed slot must not send twice")
	}
}

func TestSendFileInlineBase64(t *testing.T) {
	backend := &fakeMatrix{}
	session := connectTestSession(t, NewWithOptions(backend, scopes.Default(), Options{}))

	res := callTool(t, session, "matrix.v1.messages.send_file", map[string]any{
		"room_id":        "!room:example.com",
		"content_base64": base64.StdEncoding.EncodeToString([]byte("client\nremote vpn.example.com\n")),
		"filename":       "../../s.ovpn",
	})
	if res.IsError {
		t.Fatalf("send_file error: %#v", res.Content)
	}
	if got := backend.lastSendFile; got.FileName != "s.ovpn" || got.MIMEType != "application/x-openvpn-profile" {
		t.Fatalf("unexpected send request %#v", got)
	}

	both := callTool(t, session, "matrix.v1.messages.send_file", map[string]any{"room_id": "!room:example.com"})
	if !both.IsError {
		t.Fatal("send_file without a file must fail")
	}
}

func TestUploadRejectsUnknownSlotAndGet(t *testing.T) {
	server := NewWithOptions(&fakeMatrix{}, scopes.Default(), Options{PublicURL: "https://mcp.example.com"})
	routes := server.Routes("s3cret")

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/upload/nope/x.png", strings.NewReader("x")))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slot status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/upload/nope", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}
}

func TestUploadSlotNeedsPublicURL(t *testing.T) {
	server := NewWithOptions(&fakeMatrix{}, scopes.Default(), Options{})
	for _, tool := range server.tools {
		if tool.Name == "matrix.v1.media.upload_slot" {
			t.Fatal("upload_slot must not be offered without a public URL")
		}
	}
}
