package mcpserver

import (
	"bytes"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const mediaPathPrefix = "/media/"

// Routes serves the MCP endpoint behind bearer auth and the media download
// links without it: a link's unguessable token is its own credential.
func (s *Server) Routes(authToken string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(mediaPathPrefix, s.MediaHandler())
	mux.Handle("/", RequireBearerToken(authToken, s.Handler()))
	return mux
}

// MediaHandler streams the decrypted attachment behind a media link.
func (s *Server) MediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		target, ok := s.links.Lookup(strings.TrimPrefix(r.URL.Path, mediaPathPrefix))
		if !ok {
			setAuthResult(r, "link-invalid")
			http.NotFound(w, r)
			return
		}
		setAuthResult(r, "link")
		info, data, err := s.matrix.DownloadEventMedia(r.Context(), target.RoomID, target.EventID)
		if err != nil {
			log.Printf("media: download %s %s: %v", target.RoomID, target.EventID, err)
			http.Error(w, "media unavailable", http.StatusBadGateway)
			return
		}

		contentType := info.MIMEType
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		name := downloadFileName(info.FileName, contentType)
		disposition := "attachment"
		if inlineSafe(contentType) {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// ServeContent handles HEAD and Range requests, so browsers can seek in audio and video.
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}

// inlineSafe reports whether a browser may render the type in place: media
// only, never anything scriptable (HTML, SVG, PDF...).
func inlineSafe(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch {
	case mediaType == "image/svg+xml":
		return false
	case strings.HasPrefix(mediaType, "image/"), strings.HasPrefix(mediaType, "audio/"), strings.HasPrefix(mediaType, "video/"):
		return true
	}
	return false
}

func downloadFileName(name, contentType string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, path.Base(strings.TrimSpace(name)))
	if name == "" || name == "." || name == "/" {
		name = "attachment"
	}
	if path.Ext(name) == "" {
		name += extensionFor(contentType)
	}
	return name
}

// commonExtensions covers what messengers send; the system MIME table is not
// relied on because the distroless image has none.
var commonExtensions = map[string]string{
	"audio/ogg":       ".ogg",
	"audio/opus":      ".opus",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"audio/aac":       ".aac",
	"audio/wav":       ".wav",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

func extensionFor(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	if ext, ok := commonExtensions[mediaType]; ok {
		return ext
	}
	if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
		return exts[0]
	}
	return ""
}
