package mcpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/ricelines/matrix-mcp/internal/modules"
	"github.com/ricelines/matrix-mcp/internal/uploadslot"
)

const uploadPathPrefix = "/upload/"

type uploadResponse struct {
	UploadID string `json:"upload_id"`
	Size     int    `json:"size"`
	FileName string `json:"filename,omitempty"`
	MIMEType string `json:"mimetype,omitempty"`
}

// UploadHandler accepts the bytes of a file for an upload slot:
// PUT/POST /upload/<token>[/<filename>] with the raw file as the body (what
// `curl -T file <url>/` sends).
func (s *Server) UploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			w.Header().Set("Allow", "PUT, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		token, name, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, uploadPathPrefix), "/")
		if unescaped, err := url.PathUnescape(name); err == nil {
			name = unescaped
		}

		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, modules.MaxUploadBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				setAuthResult(r, "upload-too-large")
				http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		if len(data) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		// curl -T sends no type, other clients send octet-stream: leave
		// detection to the send in both cases.
		mimeType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mimeType == "application/octet-stream" || mimeType == "application/x-www-form-urlencoded" {
			mimeType = ""
		}

		err = s.uploads.Fill(token, uploadslot.Upload{Data: data, MIMEType: mimeType, FileName: name})
		switch {
		case errors.Is(err, uploadslot.ErrUnknown):
			setAuthResult(r, "upload-invalid")
			http.NotFound(w, r)
			return
		case errors.Is(err, uploadslot.ErrFilled):
			setAuthResult(r, "upload-filled")
			http.Error(w, "slot already holds a file; request a new slot", http.StatusConflict)
			return
		case err != nil:
			log.Printf("upload: %v", err)
			http.Error(w, "upload failed", http.StatusInternalServerError)
			return
		}
		setAuthResult(r, "upload")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(uploadResponse{UploadID: token, Size: len(data), FileName: name, MIMEType: mimeType})
	})
}
