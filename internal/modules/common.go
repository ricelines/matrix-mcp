package modules

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	matrixclient "github.com/ricelines/chat/matrix-mcp-go/internal/matrix"
)

type Audit struct {
	RequestID   string `json:"request_id" jsonschema:"server-generated request identifier"`
	TimestampMS int64  `json:"timestamp_ms" jsonschema:"unix timestamp in milliseconds"`
}

type BaseResult struct {
	OK    bool  `json:"ok" jsonschema:"true when the tool completed successfully"`
	Audit Audit `json:"audit"`
}

type Dependencies struct {
	Matrix      matrixclient.API
	Now         func() time.Time
	RequestSeed *atomic.Uint64
}

func (d Dependencies) baseResult() BaseResult {
	counter := uint64(1)
	if d.RequestSeed != nil {
		counter = d.RequestSeed.Add(1)
	}
	now := time.Now()
	if d.Now != nil {
		now = d.Now()
	}
	return BaseResult{
		OK: true,
		Audit: Audit{
			RequestID:   fmt.Sprintf("req-%d", counter),
			TimestampMS: now.UnixMilli(),
		},
	}
}

func requireNonEmpty(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
