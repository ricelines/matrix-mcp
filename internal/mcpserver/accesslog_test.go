package mcpserver

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func TestAccessLogRecordsStatusAndAuthResult(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := AccessLog(RequireBearerToken("s3cret", next))

	cases := []struct {
		header string
		want   string
	}{
		{"", "401 "},
		{"Basic s3cret", "401 "},
		{"Bearer nope", "401 "},
		{"Bearer s3cret", "202 "},
	}
	wantAuth := []string{"auth=missing", "auth=bad-scheme", "auth=bad-token", "auth=ok"}
	for i, tc := range cases {
		buf := captureLog(t)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("CF-Connecting-IP", "203.0.113.7")
		req.Header.Set("User-Agent", "test-agent")
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		handler.ServeHTTP(httptest.NewRecorder(), req)

		line := buf.String()
		for _, want := range []string{"POST / " + tc.want, "ip=203.0.113.7", `ua="test-agent"`, wantAuth[i]} {
			if !strings.Contains(line, want) {
				t.Fatalf("log line %q does not contain %q", line, want)
			}
		}
		if strings.Contains(line, "s3cret") || strings.Contains(line, "nope") {
			t.Fatalf("log line leaks credentials: %q", line)
		}
	}
}

func TestAccessLogPreservesFlusher(t *testing.T) {
	flushed := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("wrapped ResponseWriter does not implement http.Flusher")
		}
		f.Flush()
		flushed = true
	})
	captureLog(t)
	AccessLog(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !flushed {
		t.Fatal("handler did not run")
	}
}
