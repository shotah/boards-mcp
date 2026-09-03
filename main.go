package main

import (
	"fmt"
	"log"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/boards-mcp/server"
	"github.com/shotah/boards-mcp/store"
	"github.com/shotah/boards-mcp/tools"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "host-manifest" {
		if err := writeHostManifest(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if isCLI(os.Args[1:]) {
		if err := runCLI(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := runMCP(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func isCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "roster", "notices", "challenges", "help", "--help", "-h", "--version", "-version":
		return true
	default:
		return false
	}
}

func runMCP() error {
	cfg, err := store.FromEnv()
	if err != nil {
		return err
	}
	b, err := store.Open(cfg)
	if err != nil {
		return err
	}
	s := server.New()
	tools.Register(s, b)
	errLogger := log.New(os.Stderr, "", log.LstdFlags)
	return mcpserver.ServeStdio(s, mcpserver.WithErrorLogger(errLogger))
}
