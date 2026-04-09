package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestRunLog_JSON(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-a", "main")
	repo.createBranch(t, "feat-b", "feat-a")

	rootCmd.PersistentFlags().Set("json", "true")
	defer rootCmd.PersistentFlags().Set("json", "false")

	old := captureStdout(t)
	err := runLog(logCmd, nil)
	output := old.restore(t)

	if err != nil {
		t.Fatalf("runLog --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, output)
	}

	if result["trunk"] != "main" {
		t.Errorf("expected trunk=main, got %v", result["trunk"])
	}
	if result["current_branch"] == nil {
		t.Error("expected current_branch field")
	}
	if result["branches"] == nil {
		t.Error("expected branches field")
	}

	branches, ok := result["branches"].([]interface{})
	if !ok {
		t.Fatalf("branches is not an array")
	}
	// trunk + feat-a + feat-b = 3
	if len(branches) != 3 {
		t.Errorf("expected 3 branches, got %d", len(branches))
	}
}

func TestRunLog_JSON_Short_Ignored(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-x", "main")

	rootCmd.PersistentFlags().Set("json", "true")
	defer rootCmd.PersistentFlags().Set("json", "false")
	logShort = true
	defer func() { logShort = false }()

	old := captureStdout(t)
	err := runLog(logCmd, nil)
	output := old.restore(t)

	if err != nil {
		t.Fatalf("runLog --json --short failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, output)
	}
}

// stdoutCapture helps capture os.Stdout for testing JSON output.
type stdoutCapture struct {
	old *os.File
	r   *os.File
	w   *os.File
}

func captureStdout(t *testing.T) *stdoutCapture {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	return &stdoutCapture{old: old, r: r, w: w}
}

func (c *stdoutCapture) restore(t *testing.T) string {
	t.Helper()
	c.w.Close()
	os.Stdout = c.old
	var buf bytes.Buffer
	buf.ReadFrom(c.r)
	return buf.String()
}
