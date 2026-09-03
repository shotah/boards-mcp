package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shotah/boards-mcp/server"
	"github.com/shotah/boards-mcp/store"
	"github.com/shotah/boards-mcp/tools"
)

func TestRegister(t *testing.T) {
	t.Parallel()
	b, err := store.Open(store.Config{
		Path:     t.TempDir(),
		Identity: store.Identity{Author: "kit", Role: store.RoleAgent, WritesPerDay: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := server.New()
	tools.Register(s, b)
	if s == nil {
		t.Fatal("server is nil")
	}
}

func TestHostManifest(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeHostManifest(&buf); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "boards" || m["command"] != "boards-mcp" {
		t.Fatalf("%v", m)
	}
	keys, _ := m["env_keys"].([]any)
	if len(keys) != 1 || keys[0] != "BOARDS_AUTHOR" {
		t.Fatalf("env_keys = %v", m["env_keys"])
	}
}

func TestIsCLI(t *testing.T) {
	t.Parallel()
	if isCLI(nil) || isCLI([]string{}) {
		t.Fatal("empty args should be MCP stdio")
	}
	if !isCLI([]string{"roster", "list"}) {
		t.Fatal("roster is CLI")
	}
	if isCLI([]string{"--tool-tier", "core"}) {
		t.Fatal("unknown flags are not CLI (stdio MCP)")
	}
}

func TestCLIHelp(t *testing.T) {
	t.Parallel()
	if err := runCLI([]string{"help"}); err != nil {
		t.Fatal(err)
	}
}

func TestTeachTextRoundTrip(t *testing.T) {
	t.Setenv("BOARDS_AUTHOR", "")
	_, err := store.FromEnv()
	if err == nil {
		t.Fatal("expected author error")
	}
	text, ok := store.TeachText(err)
	if !ok || !strings.Contains(text, "BOARDS_AUTHOR") {
		t.Fatalf("teach = %q %v", text, ok)
	}
}
