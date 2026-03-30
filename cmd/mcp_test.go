package cmd

import (
	"testing"
)

func TestMCPCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("mcp command not registered on rootCmd")
	}
}

func TestMCPCommandFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "mcp" {
			if cmd.RunE == nil {
				t.Error("mcp command has no RunE handler")
			}
			return
		}
	}
	t.Error("mcp command not found")
}
