package modules

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
	"github.com/ricelines/matrix-mcp/internal/uploadslot"
)

// MaxUploadBytes bounds a staged file; it matches Synapse's default
// max_upload_size.
const MaxUploadBytes = 50 << 20

// maxInlineFileBytes bounds content_base64, which travels through the model.
const maxInlineFileBytes = 2 << 20

type uploadSlotInput struct{}

type uploadSlotOutput struct {
	BaseResult
	UploadID  string `json:"upload_id" jsonschema:"Pass this to matrix.v1.messages.send_file once the file is uploaded"`
	UploadURL string `json:"upload_url" jsonschema:"Capability URL; PUT the raw file bytes here, no auth header"`
	ExpiresAt string `json:"expires_at" jsonschema:"RFC 3339 expiry of the slot"`
	MaxBytes  int    `json:"max_bytes"`
	Usage     string `json:"usage"`
}

type sendFileInput struct {
	RoomID        string `json:"room_id" jsonschema:"Matrix room ID that will receive the file"`
	UploadID      string `json:"upload_id,omitempty" jsonschema:"ID of a filled upload slot (see matrix.v1.media.upload_slot); preferred for anything but tiny files"`
	ContentBase64 string `json:"content_base64,omitempty" jsonschema:"File bytes as base64, for clients without a shell; at most 2 MB decoded"`
	FileName      string `json:"filename,omitempty" jsonschema:"File name shown to the recipient; defaults to the uploaded name"`
	MIMEType      string `json:"mimetype,omitempty" jsonschema:"Media type; detected from the file name or content when empty"`
	Caption       string `json:"caption,omitempty" jsonschema:"Text shown with the file (the caption in Telegram and WhatsApp)"`
	AsFile        bool   `json:"as_file,omitempty" jsonschema:"Send images, video and audio as a document so messengers do not recompress them (keeps QR codes sharp)"`
	ReplyTo       string `json:"reply_to_event_id,omitempty" jsonschema:"Optional event ID this file replies to"`
}

func RegisterSendFile(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	if !active.Allows(scopes.ScopeMessagesSend) {
		return
	}

	if deps.Uploads != nil && deps.PublicURL != "" {
		catalog.AddTool(r, "media", scopes.ScopeMessagesSend, &mcp.Tool{
			Name: "matrix.v1.media.upload_slot",
			Description: "Get a short-lived URL to upload a local file before sending it with matrix.v1.messages.send_file. " +
				"PUT the raw bytes with curl (no auth header): `curl -sS -T <file> '<upload_url>'`. With a trailing slash in the URL curl " +
				"appends the local file name, which becomes the default name shown to the recipient. Uploading sends nothing yet.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input uploadSlotInput) (*mcp.CallToolResult, uploadSlotOutput, error) {
			token, expires, err := deps.Uploads.Create()
			if err != nil {
				return nil, uploadSlotOutput{}, err
			}
			url := deps.PublicURL + "/upload/" + token + "/"
			return nil, uploadSlotOutput{
				BaseResult: deps.baseResult(),
				UploadID:   token,
				UploadURL:  url,
				ExpiresAt:  expires.UTC().Format(time.RFC3339),
				MaxBytes:   MaxUploadBytes,
				Usage:      fmt.Sprintf("curl -sS -T <file> '%s'", url),
			}, nil
		})
	}

	catalog.AddTool(r, "messages", scopes.ScopeMessagesSend, &mcp.Tool{
		Name: "matrix.v1.messages.send_file",
		Description: "Send a file (photo, video, voice, document) into a room; bridges deliver it to Telegram/WhatsApp with the caption. " +
			"Give either upload_id (a slot filled via matrix.v1.media.upload_slot) or content_base64. Images go as photos unless as_file is set.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input sendFileInput) (*mcp.CallToolResult, messageEventOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, messageEventOutput{}, err
		}
		hasUpload, hasInline := strings.TrimSpace(input.UploadID) != "", input.ContentBase64 != ""
		if hasUpload == hasInline {
			return nil, messageEventOutput{}, errors.New("give exactly one of upload_id and content_base64")
		}

		var staged uploadslot.Upload
		if hasUpload {
			if deps.Uploads == nil {
				return nil, messageEventOutput{}, errors.New("uploads are disabled on this server")
			}
			var err error
			if staged, err = deps.Uploads.Take(input.UploadID); err != nil {
				return nil, messageEventOutput{}, err
			}
		} else {
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.ContentBase64))
			if err != nil {
				return nil, messageEventOutput{}, fmt.Errorf("content_base64: %w", err)
			}
			if len(data) > maxInlineFileBytes {
				return nil, messageEventOutput{}, fmt.Errorf("content_base64 is %d bytes, over the %d byte limit; use an upload slot", len(data), maxInlineFileBytes)
			}
			staged.Data = data
		}
		if len(staged.Data) == 0 {
			return nil, messageEventOutput{}, errors.New("the file is empty")
		}

		name := cleanFileName(input.FileName)
		if name == "" {
			name = cleanFileName(staged.FileName)
		}
		mimeType := strings.TrimSpace(input.MIMEType)
		if mimeType == "" {
			mimeType = staged.MIMEType
		}
		if mimeType == "" {
			mimeType = matrixclient.DetectMIMEType(name, staged.Data)
		}
		if name == "" {
			name = "file"
		}

		result, err := deps.Matrix.SendFile(ctx, matrixclient.SendFileRequest{
			RoomID:   input.RoomID,
			FileName: name,
			MIMEType: mimeType,
			Data:     staged.Data,
			Caption:  input.Caption,
			AsFile:   input.AsFile,
			ReplyTo:  input.ReplyTo,
		})
		if err != nil {
			if hasUpload {
				// Keep the file so the send can be retried with the same upload_id.
				deps.Uploads.Restore(input.UploadID, staged)
			}
			return nil, messageEventOutput{}, err
		}
		return nil, messageEventOutput{BaseResult: deps.baseResult(), EventID: result.EventID}, nil
	})
}

// cleanFileName keeps the last path element and drops control characters.
func cleanFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	name = path.Base(name)
	if name == "." || name == "/" {
		return ""
	}
	return name
}
