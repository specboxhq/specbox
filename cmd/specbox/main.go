package main

import (
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/storage"
	"github.com/specboxhq/specbox/mcp"
)

// Set via ldflags at build time.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "mcp":
		runMCP()
	case "version":
		fmt.Println("specbox " + version)
	case "init":
		runInit()
	case "integrate":
		runIntegrate()
	case "login":
		runLogin()
	case "whoami":
		runWhoami()
	case "update":
		fmt.Fprintln(os.Stderr, "specbox update: not yet implemented")
		os.Exit(1)
	case "push":
		fmt.Fprintln(os.Stderr, "specbox push: not yet implemented")
		os.Exit(1)
	case "pull":
		fmt.Fprintln(os.Stderr, "specbox pull: not yet implemented")
		os.Exit(1)
	case "check":
		runCheck()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: specbox <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  mcp         Start MCP server on stdio")
	fmt.Fprintln(os.Stderr, "  version     Print version")
	fmt.Fprintln(os.Stderr, "  init        Initialize specbox in current project")
	fmt.Fprintln(os.Stderr, "  integrate   Detect AI tools and configure MCP")
	fmt.Fprintln(os.Stderr, "  login       Authenticate with specbox.io")
	fmt.Fprintln(os.Stderr, "  whoami      Show current logged-in user")
	fmt.Fprintln(os.Stderr, "  update      Self-update from GitHub releases")
	fmt.Fprintln(os.Stderr, "  push        Push a spec to specbox.io")
	fmt.Fprintln(os.Stderr, "  pull        Pull responses from specbox.io")
	fmt.Fprintln(os.Stderr, "  check       Check sync status and validate spec")
}

func runMCP() {
	docsDir := resolveDocsDir()

	store, err := storage.NewFileStore(docsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	s := mcp.NewServer(store)

	if err := mcpserver.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
