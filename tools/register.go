package tools

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/boards-mcp/store"
)

// Tool names — service_verb, no server-id prefix.
// Host mcp.toml name is boards → boards__roster_list, …
const (
	ToolRosterList       = "roster_list"
	ToolRosterCreate     = "roster_create"
	ToolRosterDelete     = "roster_delete"
	ToolNoticesList      = "notices_list"
	ToolNoticesCreate    = "notices_create"
	ToolChallengesList   = "challenges_list"
	ToolChallengesGet    = "challenges_get"
	ToolChallengesCreate = "challenges_create"
	ToolChallengesUpdate = "challenges_update"
)

// ToolNames is the registered catalog (tests lock naming).
func ToolNames() []string {
	return []string{
		ToolRosterList,
		ToolRosterCreate,
		ToolRosterDelete,
		ToolNoticesList,
		ToolNoticesCreate,
		ToolChallengesList,
		ToolChallengesGet,
		ToolChallengesCreate,
		ToolChallengesUpdate,
	}
}

// Register attaches all nine tools to s.
func Register(s *mcpserver.MCPServer, b *store.Board) {
	h := &handlers{board: b}
	h.register(s)
}

type handlers struct {
	board *store.Board
}
