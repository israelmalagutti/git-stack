package mcptools

import (
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all MCP tools to the server.
func Register(s *server.MCPServer) {
	registerReadTools(s)
	registerWriteTools(s)
}
