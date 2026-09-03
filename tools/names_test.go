package tools

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/boards-mcp/server"
	"github.com/shotah/boards-mcp/store"
)

func TestToolNamesLocked(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[a-z]+_[a-z]+`)
	names := ToolNames()
	if len(names) != 9 {
		t.Fatalf("ToolNames() len = %d, want 9: %v", len(names), names)
	}
	want := map[string]bool{
		"roster_list":       true,
		"roster_create":     true,
		"roster_delete":     true,
		"notices_list":      true,
		"notices_create":    true,
		"challenges_list":   true,
		"challenges_get":    true,
		"challenges_create": true,
		"challenges_update": true,
	}
	for _, name := range names {
		if !re.MatchString(name) {
			t.Errorf("tool %q does not match ^[a-z]+_[a-z]+", name)
		}
		if strings.HasPrefix(name, "boards") {
			t.Errorf("tool %q starts with server id boards", name)
		}
		if strings.HasPrefix(name, "register") || strings.Contains(name, "remove") {
			t.Errorf("forbidden synonym %q", name)
		}
		if !want[name] {
			t.Errorf("unexpected tool %q", name)
		}
		delete(want, name)
	}
	for missing := range want {
		t.Errorf("missing tool %q", missing)
	}
}

func TestRegisteredToolNamesMatchCatalog(t *testing.T) {
	t.Parallel()
	s := newToolServer(t)
	got := registeredToolNames(t, s)
	want := ToolNames()
	if len(got) != len(want) {
		t.Fatalf("registered %d tools %v, catalog %v", len(got), keys(got), want)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("catalog tool %q is not registered", name)
		}
	}
}

func newToolServer(t *testing.T) *mcpserver.MCPServer {
	t.Helper()
	b, err := store.Open(store.Config{
		Path:     t.TempDir(),
		Identity: store.Identity{Author: "kit", Role: store.RoleAgent, WritesPerDay: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := server.New()
	Register(s, b)
	return s
}

func registeredToolNames(t *testing.T, s *mcpserver.MCPServer) map[string]bool {
	t.Helper()
	resp := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	result, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	listResult, ok := result.Result.(mcp.ListToolsResult)
	if !ok {
		t.Fatalf("expected ListToolsResult, got %T", result.Result)
	}
	names := make(map[string]bool, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		names[tool.Name] = true
	}
	return names
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
