package matrix

import (
	"context"
	"errors"
	"fmt"

	"maunium.net/go/mautrix/crypto/attachment"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// ErrNoMedia is returned when an event carries no downloadable attachment.
var ErrNoMedia = errors.New("event has no media attachment")

type MediaInfo struct {
	RoomID     string `json:"room_id"`
	EventID    string `json:"event_id"`
	Sender     string `json:"sender,omitempty"`
	MsgType    string `json:"msgtype,omitempty" jsonschema:"m.audio, m.image, m.video, m.file or the event type for stickers"`
	FileName   string `json:"filename,omitempty"`
	MIMEType   string `json:"mimetype,omitempty"`
	Size       int    `json:"size,omitempty" jsonschema:"Size in bytes as declared by the sender"`
	DurationMS int    `json:"duration_ms,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Encrypted  bool   `json:"encrypted" jsonschema:"Whether the file itself is end-to-end encrypted (decrypted by the server on download)"`
}

type eventMedia struct {
	info MediaInfo
	mxc  id.ContentURI
	file *attachment.EncryptedFile
}

// ResolveEventMedia returns the metadata of the attachment in an event
// without downloading it.
func (s *Service) ResolveEventMedia(ctx context.Context, roomID, eventID string) (MediaInfo, error) {
	media, err := s.resolveEventMedia(ctx, roomID, eventID)
	if err != nil {
		return MediaInfo{}, err
	}
	return media.info, nil
}

// DownloadEventMedia downloads the attachment of an event through the
// authenticated media API and decrypts it when it is end-to-end encrypted.
func (s *Service) DownloadEventMedia(ctx context.Context, roomID, eventID string) (MediaInfo, []byte, error) {
	media, err := s.resolveEventMedia(ctx, roomID, eventID)
	if err != nil {
		return MediaInfo{}, nil, err
	}
	data, err := s.client.DownloadBytes(ctx, media.mxc)
	if err != nil {
		return MediaInfo{}, nil, fmt.Errorf("download %s: %w", media.mxc, err)
	}
	if media.file != nil {
		if err := media.file.DecryptInPlace(data); err != nil {
			return MediaInfo{}, nil, fmt.Errorf("decrypt %s: %w", media.mxc, err)
		}
	}
	return media.info, data, nil
}

func (s *Service) resolveEventMedia(ctx context.Context, roomID, eventID string) (eventMedia, error) {
	evt, err := s.client.GetEvent(ctx, id.RoomID(roomID), id.EventID(eventID))
	if err != nil {
		return eventMedia{}, fmt.Errorf("get event: %w", err)
	}
	evt, err = s.decryptEvent(ctx, evt)
	if err != nil {
		return eventMedia{}, err
	}
	if evt.Type != event.EventMessage && evt.Type != event.EventSticker {
		return eventMedia{}, fmt.Errorf("%w: event type is %s", ErrNoMedia, evt.Type.Type)
	}
	if err := ensureParsedEventContent(evt); err != nil {
		return eventMedia{}, fmt.Errorf("parse event %s: %w", evt.ID, err)
	}
	content := evt.Content.AsMessage()

	media := eventMedia{info: MediaInfo{
		RoomID:  roomID,
		EventID: eventID,
		Sender:  evt.Sender.String(),
		MsgType: string(content.MsgType),
	}}
	if evt.Type == event.EventSticker {
		media.info.MsgType = evt.Type.Type
	}
	switch {
	case content.File != nil:
		media.mxc, err = content.File.URL.Parse()
		media.file = &content.File.EncryptedFile
		media.info.Encrypted = true
	case content.URL != "":
		media.mxc, err = content.URL.Parse()
	default:
		return eventMedia{}, ErrNoMedia
	}
	if err != nil {
		return eventMedia{}, fmt.Errorf("parse media URI: %w", err)
	}

	// Per spec, body is the file name when no separate filename (caption) is given.
	media.info.FileName = content.FileName
	if media.info.FileName == "" {
		media.info.FileName = content.Body
	}
	if content.Info != nil {
		media.info.MIMEType = content.Info.MimeType
		media.info.Size = content.Info.Size
		media.info.DurationMS = content.Info.Duration
		media.info.Width = content.Info.Width
		media.info.Height = content.Info.Height
	}
	return media, nil
}
