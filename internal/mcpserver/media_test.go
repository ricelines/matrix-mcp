package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

func callMediaDownload(t *testing.T, server *Server) *mcp.CallToolResult {
	t.Helper()
	session := connectTestSession(t, server)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "matrix.v1.media.download",
		Arguments: map[string]any{"room_id": "!room:example.com", "event_id": "$voice"},
	})
	if err != nil {
		t.Fatalf("media.download error = %v", err)
	}
	if res.IsError {
		t.Fatalf("media.download returned an error result: %#v", res.Content)
	}
	return res
}

func TestMediaDownloadIssuesWorkingLink(t *testing.T) {
	backend := &fakeMatrix{
		mediaInfo: matrixclient.MediaInfo{RoomID: "!room:example.com", EventID: "$voice", MsgType: "m.audio", MIMEType: "audio/ogg", FileName: "Голосовое", Encrypted: true},
		mediaData: []byte("OggS voice"),
	}
	server := NewWithOptions(backend, scopes.Default(), Options{PublicURL: "https://mcp.example.com"})

	res := callMediaDownload(t, server)
	out := structuredMap(t, res)
	link, _ := out["download_url"].(string)
	if !strings.HasPrefix(link, "https://mcp.example.com/media/") || out["expires_at"] == "" || out["image_attached"] != false {
		t.Fatalf("unexpected output %#v", out)
	}
	if len(res.Content) != 1 {
		t.Fatalf("audio must not be attached inline, got %d content blocks", len(res.Content))
	}
	if backend.mediaDownloads != 0 {
		t.Fatalf("audio was downloaded %d times while only issuing a link", backend.mediaDownloads)
	}

	// The link works without the bearer token, while the MCP endpoint still requires it.
	routes := AccessLog(server.Routes("s3cret"))
	path := strings.TrimPrefix(link, "https://mcp.example.com")
	rec := httptest.NewRecorder()
	captureLog(t)
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET link status = %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "OggS voice" {
		t.Fatalf("GET link body = %q", body)
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/ogg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline;") || !strings.Contains(got, "filename*=utf-8''") {
		t.Fatalf("Content-Disposition = %q, want inline with an RFC 2231 file name", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}

	rec = httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("MCP endpoint without token = %d, want 401", rec.Code)
	}
}

func TestMediaLinkRejectsUnknownTokensAndOtherMethods(t *testing.T) {
	server := NewWithOptions(&fakeMatrix{}, scopes.Default(), Options{PublicURL: "https://mcp.example.com"})
	routes := server.Routes("s3cret")

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/not-a-token", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token status = %d, want 404", rec.Code)
	}
	rec = httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/media/whatever", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}

func TestMediaDownloadAttachesSupportedImages(t *testing.T) {
	backend := &fakeMatrix{
		mediaInfo: matrixclient.MediaInfo{MsgType: "m.image", MIMEType: "image/png", FileName: "photo.png", Size: 9},
		mediaData: []byte("\x89PNG data"),
	}
	res := callMediaDownload(t, NewWithOptions(backend, scopes.Default(), Options{PublicURL: "https://mcp.example.com"}))
	if len(res.Content) != 2 {
		t.Fatalf("got %d content blocks, want text + image", len(res.Content))
	}
	image, ok := res.Content[1].(*mcp.ImageContent)
	if !ok || image.MIMEType != "image/png" || string(image.Data) != "\x89PNG data" {
		t.Fatalf("second block = %#v, want the PNG", res.Content[1])
	}
	if structuredMap(t, res)["image_attached"] != true {
		t.Fatal("image_attached should be true")
	}

	// Formats Claude cannot read (HEIC) only get a link.
	backend = &fakeMatrix{mediaInfo: matrixclient.MediaInfo{MsgType: "m.image", MIMEType: "image/heic"}, mediaData: []byte("heic")}
	res = callMediaDownload(t, NewWithOptions(backend, scopes.Default(), Options{PublicURL: "https://mcp.example.com"}))
	if len(res.Content) != 1 || backend.mediaDownloads != 0 {
		t.Fatalf("HEIC: %d blocks, %d downloads; want link only", len(res.Content), backend.mediaDownloads)
	}
}

func TestMediaDownloadWithoutPublicURLExplains(t *testing.T) {
	backend := &fakeMatrix{mediaInfo: matrixclient.MediaInfo{MsgType: "m.file", MIMEType: "application/pdf"}}
	out := structuredMap(t, callMediaDownload(t, New(backend, scopes.Default())))
	if out["download_url"] != nil || !strings.Contains(out["note"].(string), "MATRIX_MCP_PUBLIC_URL") {
		t.Fatalf("unexpected output %#v", out)
	}
}

func TestDownloadFileNameAndInlineSafety(t *testing.T) {
	cases := []struct{ name, ctype, want string }{
		{"", "audio/ogg", "attachment.ogg"},
		{"../../etc/passwd", "text/plain", "passwd.txt"},
		{"voice\nnote.ogg", "audio/ogg", "voicenote.ogg"},
		{"photo.jpg", "image/jpeg", "photo.jpg"},
	}
	for _, tc := range cases {
		got := downloadFileName(tc.name, tc.ctype)
		if !strings.HasPrefix(got, strings.TrimSuffix(tc.want, ".txt")) {
			t.Errorf("downloadFileName(%q, %q) = %q, want %q", tc.name, tc.ctype, got, tc.want)
		}
	}
	for ctype, want := range map[string]bool{
		"audio/ogg": true, "video/mp4": true, "image/png": true,
		"image/svg+xml": false, "text/html": false, "application/pdf": false, "": false,
	} {
		if got := inlineSafe(ctype); got != want {
			t.Errorf("inlineSafe(%q) = %v, want %v", ctype, got, want)
		}
	}
}

func TestAccessLogRedactsMediaTokens(t *testing.T) {
	if got := redactPath("/media/abcdef"); got != "/media/<token>" {
		t.Fatalf("redactPath = %q", got)
	}
	if got := redactPath("/"); got != "/" {
		t.Fatalf("redactPath(/) = %q", got)
	}
}
