package cmdutil

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd(json, debug, noInteractive bool) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	root := &cobra.Command{Use: "gs"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Bool("no-interactive", false, "")
	root.AddCommand(cmd)
	if json {
		_ = root.PersistentFlags().Set("json", "true")
	}
	if debug {
		_ = root.PersistentFlags().Set("debug", "true")
	}
	if noInteractive {
		_ = root.PersistentFlags().Set("no-interactive", "true")
	}
	return cmd
}

func TestJSONMode(t *testing.T) {
	cmd := newTestCmd(true, false, false)
	if !JSONMode(cmd) {
		t.Error("expected JSONMode to return true when --json is set")
	}

	cmd = newTestCmd(false, false, false)
	if JSONMode(cmd) {
		t.Error("expected JSONMode to return false when --json is not set")
	}
}

func TestDebugMode(t *testing.T) {
	cmd := newTestCmd(false, true, false)
	if !DebugMode(cmd) {
		t.Error("expected DebugMode to return true when --debug is set")
	}

	cmd = newTestCmd(false, false, false)
	if DebugMode(cmd) {
		t.Error("expected DebugMode to return false when --debug is not set")
	}
}

func TestInteractiveMode(t *testing.T) {
	// Default is true (interactive)
	cmd := newTestCmd(false, false, false)
	if !InteractiveMode(cmd) {
		t.Error("expected InteractiveMode to return true by default")
	}

	// --json disables interactive
	cmd = newTestCmd(true, false, false)
	if InteractiveMode(cmd) {
		t.Error("expected InteractiveMode to return false when --json is set")
	}

	// --no-interactive disables interactive
	cmd = newTestCmd(false, false, true)
	if InteractiveMode(cmd) {
		t.Error("expected InteractiveMode to return false when --no-interactive is set")
	}
}

func TestPrintJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]string{"key": "value"}
	err := PrintJSON(data)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "\"key\": \"value\"") {
		t.Errorf("expected indented JSON, got: %s", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "}") {
		t.Errorf("expected JSON object, got: %s", output)
	}
}

func TestPrintJSONError(t *testing.T) {
	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintJSONError("test-cmd", errors.New("something failed"))

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "\"error\": \"something failed\"") {
		t.Errorf("expected error field in JSON, got: %s", output)
	}
	if !strings.Contains(output, "\"command\": \"test-cmd\"") {
		t.Errorf("expected command field in JSON, got: %s", output)
	}
}

func TestDebugf_Enabled(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := newTestCmd(false, true, false)
	Debugf(cmd, "hello %s", "world")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "[debug]") {
		t.Errorf("expected [debug] prefix, got: %s", output)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected message 'hello world', got: %s", output)
	}
}

func TestDebugf_Disabled(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := newTestCmd(false, false, false)
	Debugf(cmd, "should not appear")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if output != "" {
		t.Errorf("expected no output when debug is off, got: %s", output)
	}
}
