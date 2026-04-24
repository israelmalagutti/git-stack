package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
)

func TestGlobalFlags_Registered(t *testing.T) {
	for _, name := range []string{"json", "debug", "no-interactive"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("expected persistent flag %q on root command", name)
		}
	}
}

func TestGlobalFlags_JSONMode(t *testing.T) {
	_ = rootCmd.PersistentFlags().Set("json", "true")
	defer func() { _ = rootCmd.PersistentFlags().Set("json", "false") }()

	if !cmdutil.JSONMode(rootCmd) {
		t.Fatal("expected JSONMode true")
	}
}

func TestGlobalFlags_DebugMode(t *testing.T) {
	_ = rootCmd.PersistentFlags().Set("debug", "true")
	defer func() { _ = rootCmd.PersistentFlags().Set("debug", "false") }()

	if !cmdutil.DebugMode(rootCmd) {
		t.Fatal("expected DebugMode true")
	}
}
