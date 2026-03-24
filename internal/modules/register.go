package modules

import (
	"sync/atomic"
	"time"

	"github.com/ricelines/matrix-mcp/internal/catalog"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

func RegisterAll(r *catalog.Registrar, matrix matrixclient.API, active scopes.Set) {
	deps := Dependencies{
		Matrix:      matrix,
		Now:         time.Now,
		RequestSeed: &atomic.Uint64{},
	}
	RegisterClient(r, deps, active)
	RegisterServer(r, deps, active)
	RegisterUsers(r, deps, active)
	RegisterRooms(r, deps, active)
	RegisterRoomMembers(r, deps, active)
	RegisterRoomState(r, deps, active)
	RegisterTimeline(r, deps, active)
	RegisterMessages(r, deps, active)
}
