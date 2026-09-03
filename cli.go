package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/shotah/boards-mcp/server"
	"github.com/shotah/boards-mcp/store"
)

func runCLI(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(`boards-mcp — stdio MCP (no args) or debug CLI.

  boards-mcp                         # MCP stdio
  boards-mcp host-manifest
  boards-mcp roster list
  boards-mcp roster create --agent-name Kit --user-name Chris
  boards-mcp roster delete
  boards-mcp notices list
  boards-mcp challenges list
  boards-mcp challenges get <id>
  boards-mcp --version

Env: BOARDS_AUTHOR (required), BOARDS_PATH, BOARDS_ROLE, BOARDS_WRITES_PER_DAY
`)
		return nil
	}
	if args[0] == "--version" || args[0] == "-version" {
		fmt.Println(server.ServerVersion)
		return nil
	}
	cfg, err := store.FromEnv()
	if err != nil {
		return err
	}
	b, err := store.Open(cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "roster":
		return cliRoster(b, args[1:])
	case "notices":
		return cliNotices(b, args[1:])
	case "challenges":
		return cliChallenges(b, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func cliRoster(b *store.Board, args []string) error {
	if len(args) == 0 {
		return errors.New("roster list|create|delete")
	}
	switch args[0] {
	case "list":
		rows, err := b.RosterList()
		return printJSON(rows, err)
	case "delete":
		if err := b.RosterDelete(); err != nil {
			return err
		}
		return printJSON(map[string]any{"ok": true}, nil)
	case "create":
		agent, user := flagVal(args[1:], "--agent-name"), flagVal(args[1:], "--user-name")
		row, err := b.RosterCreate(agent, user)
		return printJSON(row, err)
	default:
		return errors.New("roster list|create|delete")
	}
}

func cliNotices(b *store.Board, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		rows, err := b.NoticesList(25)
		return printJSON(rows, err)
	}
	return errors.New("notices list")
}

func cliChallenges(b *store.Board, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		rows, err := b.ChallengesList(25, true)
		return printJSON(rows, err)
	}
	if args[0] == "get" {
		if len(args) < 2 {
			return errors.New("challenges get <id>")
		}
		view, err := b.ChallengesGet(args[1])
		return printJSON(view, err)
	}
	return errors.New("challenges list|get")
}

func flagVal(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func printJSON(v any, err error) error {
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
