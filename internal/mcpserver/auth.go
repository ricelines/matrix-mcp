package mcpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireBearerToken rejects requests that do not carry
// "Authorization: Bearer <token>". An empty token disables the check.
func RequireBearerToken(token string, next http.Handler) http.Handler {
	if token == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setAuthResult(r, "disabled")
			next.ServeHTTP(w, r)
		})
	}
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		scheme, got, ok := strings.Cut(header, " ")
		// Compare fixed-size digests so the check is constant-time regardless of input length.
		gotSum := sha256.Sum256([]byte(strings.TrimSpace(got)))
		var reason string
		switch {
		case header == "":
			reason = "missing"
		case !ok || !strings.EqualFold(scheme, "Bearer"):
			reason = "bad-scheme"
		case subtle.ConstantTimeCompare(gotSum[:], want[:]) != 1:
			reason = "bad-token"
		}
		if reason != "" {
			setAuthResult(r, reason)
			w.Header().Set("WWW-Authenticate", `Bearer realm="matrix-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		setAuthResult(r, "ok")
		next.ServeHTTP(w, r)
	})
}
