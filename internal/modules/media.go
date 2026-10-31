package modules

import (
	"context"
	"encoding/json"
	"mime"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/medialink"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

// maxInlineImageBytes bounds images returned inside the tool result.
const maxInlineImageBytes = 5 << 20

// inlineImageTypes are the image formats MCP clients such as Claude accept.
var inlineImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

type mediaDownloadInput struct {
	RoomID    string `json:"room_id" jsonschema:"Matrix room ID containing the event"`
	EventID   string `json:"event_id" jsonschema:"Event ID of the message carrying the attachment (m.audio, m.image, m.video, m.file or a sticker)"`
	SkipImage bool   `json:"skip_image,omitempty" jsonschema:"Do not attach the image itself to the result, only metadata and the download link"`
}

type mediaDownloadOutput struct {
	BaseResult
	Media         matrixclient.MediaInfo `json:"media"`
	DownloadURL   string                 `json:"download_url,omitempty" jsonschema:"Short-lived link to the decrypted file; works without the MCP auth header (browser, curl) until expires_at"`
	ExpiresAt     string                 `json:"expires_at,omitempty" jsonschema:"RFC 3339 expiry of download_url"`
	ImageAttached bool                   `json:"image_attached" jsonschema:"Whether the image itself is attached to this result as image content"`
	Note          string                 `json:"note,omitempty"`
}

func RegisterMedia(r *catalog.Registrar, deps Dependencies, active scopes.Set) {
	r.AddModule("media", "Download and decrypt event attachments: voice messages, images, video, files.")

	if !active.Allows(scopes.ScopeMediaRead) {
		return
	}
	catalog.AddTool(r, "media", scopes.ScopeMediaRead, &mcp.Tool{
		Name: "matrix.v1.media.download",
		Description: "Fetch the attachment of a message (voice message, photo, video, file), decrypting end-to-end encrypted media. " +
			"Returns metadata and a short-lived download_url that needs no auth header. Supported images (JPEG/PNG/GIF/WebP up to 5 MB) " +
			"are also attached to the result so they can be looked at directly. Audio and video cannot be played by the model: give " +
			"download_url to the user, or fetch it with a tool that can transcribe.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mediaDownloadInput) (*mcp.CallToolResult, mediaDownloadOutput, error) {
		if err := requireNonEmpty("room_id", input.RoomID); err != nil {
			return nil, mediaDownloadOutput{}, err
		}
		if err := requireNonEmpty("event_id", input.EventID); err != nil {
			return nil, mediaDownloadOutput{}, err
		}
		info, err := deps.Matrix.ResolveEventMedia(ctx, input.RoomID, input.EventID)
		if err != nil {
			return nil, mediaDownloadOutput{}, err
		}

		out := mediaDownloadOutput{BaseResult: deps.baseResult(), Media: info}
		if deps.MediaLinks != nil && deps.PublicURL != "" {
			token, expires, err := deps.MediaLinks.Create(medialink.Target{RoomID: input.RoomID, EventID: input.EventID})
			if err != nil {
				return nil, mediaDownloadOutput{}, err
			}
			out.DownloadURL = deps.PublicURL + "/media/" + token
			out.ExpiresAt = expires.UTC().Format(time.RFC3339)
		} else {
			out.Note = "download links are disabled: the server has no public URL configured (MATRIX_MCP_PUBLIC_URL)"
		}

		// Senders may add parameters (e.g. "; charset"); clients want the bare type.
		imageType, _, _ := mime.ParseMediaType(info.MIMEType)
		if input.SkipImage || !inlineImageTypes[imageType] || info.Size > maxInlineImageBytes {
			return nil, out, nil
		}
		_, data, err := deps.Matrix.DownloadEventMedia(ctx, input.RoomID, input.EventID)
		if err != nil {
			return nil, mediaDownloadOutput{}, err
		}
		if len(data) > maxInlineImageBytes {
			return nil, out, nil
		}
		out.ImageAttached = true
		text, err := json.Marshal(out)
		if err != nil {
			return nil, mediaDownloadOutput{}, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: string(text)},
			&mcp.ImageContent{Data: data, MIMEType: imageType},
		}}, out, nil
	})
}
