package cmd

import (
	"github.com/israelmalagutti/git-stack/internal/mcptools"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI agent integration (stdio)",
	Long: `Start a Model Context Protocol server over stdio.

Used by AI coding agents (Claude Code, Cursor, etc.) to interact with
git-stack programmatically. The server exposes stack operations as MCP
tools and returns structured JSON instead of terminal output.

Configuration example for Claude Code (~/.claude/settings.json):

  {
    "mcpServers": {
      "git-stack": {
        "command": "gs",
        "args": ["mcp"]
      }
    }
  }`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	s := server.NewMCPServer(
		"git-stack",
		Version,
		server.WithToolCapabilities(false),
	)

	mcptools.Register(s)

	return server.ServeStdio(s)
}
