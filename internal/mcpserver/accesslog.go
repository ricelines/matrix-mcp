package mcpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type requestInfoKey struct{}

// requestInfo is filled in by inner middleware so the access log can report it.
type requestInfo struct {
	auth string
}

func setAuthResult(r *http.Request, result string) {
	if info, ok := r.Context().Value(requestInfoKey{}).(*requestInfo); ok {
		info.auth = result
	}
}

// AccessLog logs one line per request: method, path, status, duration, client
// IP, user agent and the auth outcome. Credentials are never logged.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := &requestInfo{auth: "-"}
		r = r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info))
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("http: %s %s %d %s ip=%s ua=%q auth=%s",
			r.Method, r.URL.Path, status, time.Since(start).Round(time.Millisecond), clientIP(r), r.UserAgent(), info.auth)
	})
}

// clientIP prefers the address reported by Cloudflare / a reverse proxy. It is
// informational only and must not be used for access decisions.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Flush keeps SSE streaming working: the MCP SDK type-asserts http.Flusher.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
