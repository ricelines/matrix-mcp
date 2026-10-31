package matrix

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"path"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/attachment"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type SendFileRequest struct {
	RoomID   string
	FileName string
	MIMEType string
	Data     []byte
	// Caption is shown under the file. Bridges pass it on as the Telegram or
	// WhatsApp caption.
	Caption string
	// AsFile sends images, video and audio as a plain file (m.file), so a
	// bridge does not recompress them (a QR code stays sharp).
	AsFile bool
	// ReplyTo is an optional event ID the file replies to.
	ReplyTo string
}

// extensionTypes covers what is commonly sent; the system MIME table is not
// relied on because the distroless image has none.
var extensionTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".heic": "image/heic",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".ogg":  "audio/ogg",
	".opus": "audio/opus",
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".wav":  "audio/wav",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".json": "application/json",
	".txt":  "text/plain",
	".conf": "text/plain",
	".ovpn": "application/x-openvpn-profile",
	".vpn":  "text/plain",
}

// DetectMIMEType picks a media type from the file name, falling back to
// content sniffing.
func DetectMIMEType(fileName string, data []byte) string {
	ext := strings.ToLower(path.Ext(fileName))
	if t, ok := extensionTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); ext != "" && t != "" {
		return t
	}
	return http.DetectContentType(data)
}

func fileMsgType(mimeType string, asFile bool) event.MessageType {
	mediaType, _, _ := mime.ParseMediaType(mimeType)
	switch {
	case asFile, mediaType == "image/svg+xml":
		return event.MsgFile
	case strings.HasPrefix(mediaType, "image/"):
		return event.MsgImage
	case strings.HasPrefix(mediaType, "video/"):
		return event.MsgVideo
	case strings.HasPrefix(mediaType, "audio/"):
		return event.MsgAudio
	}
	return event.MsgFile
}

// buildFileContent returns the message content for a file and the bytes to
// upload. In an encrypted room the file is encrypted first and the content
// carries the key; the caller fills in the uploaded URI with setFileURI.
func buildFileContent(req SendFileRequest, encrypted bool) (*event.MessageEventContent, []byte) {
	mimeType := req.MIMEType
	if mimeType == "" {
		mimeType = DetectMIMEType(req.FileName, req.Data)
	}
	content := &event.MessageEventContent{
		MsgType: fileMsgType(mimeType, req.AsFile),
		Body:    req.FileName,
		Info:    &event.FileInfo{MimeType: mimeType, Size: len(req.Data)},
	}
	// MSC2530 / spec 1.10: a body that differs from filename is the caption.
	if caption := strings.TrimSpace(req.Caption); caption != "" {
		content.Body = caption
		content.FileName = req.FileName
	}
	if content.MsgType == event.MsgImage {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(req.Data)); err == nil {
			content.Info.Width, content.Info.Height = cfg.Width, cfg.Height
		}
	}

	if !encrypted {
		return content, req.Data
	}
	file := attachment.NewEncryptedFile()
	ciphertext := file.Encrypt(req.Data)
	content.File = &event.EncryptedFileInfo{EncryptedFile: *file}
	return content, ciphertext
}

func setFileURI(content *event.MessageEventContent, uri id.ContentURI) {
	if content.File != nil {
		content.File.URL = uri.CUString()
		return
	}
	content.URL = uri.CUString()
}

func (s *Service) roomEncrypted(ctx context.Context, roomID id.RoomID) (bool, error) {
	if s.client == nil || s.client.StateStore == nil || s.client.Crypto == nil {
		return false, nil
	}
	return s.client.StateStore.IsEncrypted(ctx, roomID)
}

// SendFile uploads a file (encrypting it in encrypted rooms) and posts it as
// an image, video, audio or file message.
func (s *Service) SendFile(ctx context.Context, req SendFileRequest) (EventWriteResult, error) {
	roomID := id.RoomID(req.RoomID)
	if err := s.ensureRoomSendState(ctx, roomID); err != nil {
		return EventWriteResult{}, fmt.Errorf("prepare room for send: %w", err)
	}
	encrypted, err := s.roomEncrypted(ctx, roomID)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("check room encryption state: %w", err)
	}

	var original *event.Event
	if req.ReplyTo != "" {
		original, err = s.client.GetEvent(ctx, roomID, id.EventID(req.ReplyTo))
		if err != nil {
			return EventWriteResult{}, fmt.Errorf("load replied-to event: %w", err)
		}
	}

	content, payload := buildFileContent(req, encrypted)
	upload := mautrix.ReqUploadMedia{ContentBytes: payload, ContentType: content.Info.MimeType, FileName: req.FileName}
	if encrypted {
		// Neither the type nor the name of an encrypted file goes to the media repo.
		upload.ContentType, upload.FileName = "application/octet-stream", ""
	}
	uploaded, err := s.client.UploadMedia(ctx, upload)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("upload media: %w", err)
	}
	setFileURI(content, uploaded.ContentURI)
	if original != nil {
		content.SetReply(original)
	}

	resp, err := s.client.SendMessageEvent(ctx, roomID, event.EventMessage, content)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("send file: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}
