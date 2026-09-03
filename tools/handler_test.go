package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/boards-mcp/server"
	"github.com/shotah/boards-mcp/store"
)

func TestRosterThenChallengeByIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	maya := boardAt(t, dir, "maya", now)
	kit := boardAt(t, dir, "kit", now)
	sMaya := serverWith(t, maya)
	sKit := serverWith(t, kit)

	text, isErr := callTool(t, sMaya, ToolRosterCreate, map[string]any{"agent_name": "Maya", "user_name": "Sister"})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sKit, ToolRosterCreate, map[string]any{"agent_name": "Kit", "user_name": "Chris"})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sMaya, ToolRosterList, map[string]any{})
	if isErr || !strings.Contains(text, "Chris") {
		t.Fatalf("roster_list = %s err=%v", text, isErr)
	}
	text, isErr = callTool(t, sMaya, ToolChallengesCreate, map[string]any{
		"title":        "100k steps",
		"kind":         "steps",
		"mode":         "sum",
		"target":       100000,
		"window_days":  14,
		"participants": "maya,kit",
	})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sKit, ToolChallengesList, map[string]any{})
	if isErr || !strings.Contains(text, `"items"`) || !strings.Contains(text, "100k") {
		t.Fatalf("list = %s err=%v", text, isErr)
	}
	var listed WatchResult
	if err := json.Unmarshal([]byte(text), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("items = %+v", listed)
	}
	id := listed.Items[0].ID
	text, isErr = callTool(t, sKit, ToolChallengesGet, map[string]any{"challenge_id": id})
	if isErr || !strings.Contains(text, "100k") {
		t.Fatalf("get = %s err=%v", text, isErr)
	}
	text, isErr = callTool(t, sKit, ToolChallengesUpdate, map[string]any{
		"challenge_id": id, "action": "accept",
	})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sKit, ToolChallengesUpdate, map[string]any{
		"challenge_id": id, "action": "check_in", "value": 9000.0, "proof": "garmin:steps:2026-09-03",
	})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sMaya, ToolNoticesCreate, map[string]any{"body": "pinned"})
	if isErr {
		t.Fatal(text)
	}
	text, isErr = callTool(t, sMaya, ToolRosterDelete, map[string]any{})
	if isErr {
		t.Fatal(text)
	}
}

func TestRosterDeleteTeachIn(t *testing.T) {
	t.Parallel()
	s := serverWith(t, boardAt(t, t.TempDir(), "kit", time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)))
	text, isErr := callTool(t, s, ToolRosterDelete, map[string]any{})
	if !isErr || !strings.Contains(text, "not on the roster") {
		t.Fatalf("delete teach-in = %s err=%v", text, isErr)
	}
}

func TestWatchShape(t *testing.T) {
	t.Parallel()
	b := boardAt(t, t.TempDir(), "kit", time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC))
	if _, err := b.RosterCreate("Kit", "Chris"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NoticesCreate("hello"); err != nil {
		t.Fatal(err)
	}
	s := serverWith(t, b)
	text, isErr := callTool(t, s, ToolNoticesList, map[string]any{})
	if isErr {
		t.Fatal(text)
	}
	var got WatchResult
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID == "" || !strings.HasPrefix(got.Items[0].URL, "boards:notice/") {
		t.Fatalf("%+v", got)
	}
}

func boardAt(t *testing.T, dir, author string, now time.Time) *store.Board {
	t.Helper()
	b, err := store.Open(store.Config{
		Path:     dir,
		Identity: store.Identity{Author: author, Role: store.RoleAgent, WritesPerDay: 12},
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func serverWith(t *testing.T, b *store.Board) *mcpserver.MCPServer {
	t.Helper()
	s := server.New()
	Register(s, b)
	return s
}

func callTool(t *testing.T, s *mcpserver.MCPServer, toolName string, args map[string]any) (text string, isError bool) {
	t.Helper()
	params := map[string]any{"name": toolName, "arguments": args}
	msg := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := s.HandleMessage(context.Background(), raw)
	switch r := resp.(type) {
	case mcp.JSONRPCResponse:
		result, ok := r.Result.(*mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected *CallToolResult, got %T", r.Result)
		}
		if len(result.Content) == 0 {
			return "", result.IsError
		}
		tc, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("expected TextContent, got %T", result.Content[0])
		}
		return tc.Text, result.IsError
	case mcp.JSONRPCError:
		t.Fatalf("protocol error %d: %s", r.Error.Code, r.Error.Message)
		return "", true
	default:
		t.Fatalf("unexpected response type %T", resp)
		return "", true
	}
}
