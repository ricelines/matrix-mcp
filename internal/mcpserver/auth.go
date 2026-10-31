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
		return next
	}
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, got, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		// Compare fixed-size digests so the check is constant-time regardless of input length.
		gotSum := sha256.Sum256([]byte(strings.TrimSpace(got)))
		if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare(gotSum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="matrix-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
