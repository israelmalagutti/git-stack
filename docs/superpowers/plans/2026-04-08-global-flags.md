# Global `--json`, `--debug`, `--no-interactive` Flags — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add global `--json`, `--debug`, and `--no-interactive` flags to the `gs` CLI. Prove the architecture works end-to-end by converting `gs log` (the simplest read command) to JSON-first output.

**Architecture:** Every command will produce a typed result struct. `--json` marshals it to stdout. Without `--json`, a human formatter renders it as colored text. `--debug` writes timestamped lines to stderr independently. `--json` implies `--no-interactive` and disables color. Infrastructure goes in a new `internal/cmdutil/` package to avoid circular deps between `cmd/` and output helpers.

**Tech Stack:** Go, Cobra (PersistentFlags), `encoding/json`, existing `internal/colors` package.

**Spec:** `docs/json-debug-flags-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| **Create:** `internal/cmdutil/output.go` | `printJSON()`, `debugf()`, `jsonMode()`, `debugMode()`, `interactiveMode()` helpers that read flag state from `*cobra.Command` |
| **Create:** `internal/cmdutil/output_test.go` | Tests for output helpers |
| **Modify:** `cmd/root.go` | Add `--json`, `--debug`, `--no-interactive` as PersistentFlags; update `Execute()` to handle JSON error output |
| **Create:** `cmd/root_flags_test.go` | Tests for global flag wiring and JSON error output |
| **Modify:** `cmd/log.go` | Refactor `runLog` to produce a result struct, format based on `--json` flag |
| **Create:** `cmd/log_json_test.go` | Tests for `gs log --json` output |

---

### Task 1: Create `internal/cmdutil/output.go` — flag reader helpers

**Files:**
- Create: `internal/cmdutil/output.go`
- Create: `internal/cmdutil/output_test.go`

- [ ] **Step 1: Write failing tests for flag reader helpers**

Create `internal/cmdutil/output_test.go`:

```go
package cmdutil

import (
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
		root.PersistentFlags().Set("json", "true")
	}
	if debug {
		root.PersistentFlags().Set("debug", "true")
	}
	if noInteractive {
		root.PersistentFlags().Set("no-interactive", "true")
	}
	return cmd
}

func TestJSONMode(t *testing.T) {
	cmd := newTestCmd(true, false, false)
	if !JSONMode(cmd) {
		t.Fatal("expected JSONMode true")
	}

	cmd = newTestCmd(false, false, false)
	if JSONMode(cmd) {
		t.Fatal("expected JSONMode false")
	}
}

func TestDebugMode(t *testing.T) {
	cmd := newTestCmd(false, true, false)
	if !DebugMode(cmd) {
		t.Fatal("expected DebugMode true")
	}
}

func TestInteractiveMode(t *testing.T) {
	// --no-interactive explicitly set
	cmd := newTestCmd(false, false, true)
	if InteractiveMode(cmd) {
		t.Fatal("expected InteractiveMode false with --no-interactive")
	}

	// --json implies non-interactive
	cmd = newTestCmd(true, false, false)
	if InteractiveMode(cmd) {
		t.Fatal("expected InteractiveMode false with --json")
	}

	// default: interactive
	cmd = newTestCmd(false, false, false)
	if !InteractiveMode(cmd) {
		t.Fatal("expected InteractiveMode true by default")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmdutil/ -v -run 'TestJSONMode|TestDebugMode|TestInteractiveMode'`
Expected: Compilation error — package does not exist yet.

- [ ] **Step 3: Implement the helpers**

Create `internal/cmdutil/output.go`:

```go
package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// JSONMode returns true if --json flag is set on this command or any parent.
func JSONMode(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

// DebugMode returns true if --debug flag is set.
func DebugMode(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("debug")
	return v
}

// InteractiveMode returns true unless --no-interactive or --json is set.
func InteractiveMode(cmd *cobra.Command) bool {
	if JSONMode(cmd) {
		return false
	}
	v, _ := cmd.Flags().GetBool("no-interactive")
	return !v
}

// PrintJSON marshals data as indented JSON to stdout.
func PrintJSON(data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

// PrintJSONError prints a JSON error object to stderr and returns nil
// (the error has been handled by printing it).
func PrintJSONError(command string, err error) {
	resp := struct {
		Error   string `json:"error"`
		Command string `json:"command,omitempty"`
	}{
		Error:   err.Error(),
		Command: command,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(os.Stderr, string(b))
}

// Debugf writes a timestamped debug line to stderr if --debug is set.
func Debugf(cmd *cobra.Command, format string, args ...interface{}) {
	if !DebugMode(cmd) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s [debug] %s\n", time.Now().Format(time.RFC3339), msg)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cmdutil/ -v -run 'TestJSONMode|TestDebugMode|TestInteractiveMode'`
Expected: PASS

- [ ] **Step 5: Write tests for PrintJSON and Debugf**

Add to `internal/cmdutil/output_test.go`:

```go
func TestPrintJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := struct {
		Name string `json:"name"`
	}{Name: "test"}
	if err := PrintJSON(data); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, `"name": "test"`) {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestPrintJSONError(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintJSONError("checkout", fmt.Errorf("branch not found"))

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, `"error": "branch not found"`) {
		t.Fatalf("unexpected error output: %s", output)
	}
	if !strings.Contains(output, `"command": "checkout"`) {
		t.Fatalf("expected command field: %s", output)
	}
}

func TestDebugf_Enabled(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := newTestCmd(false, true, false)
	Debugf(cmd, "loading %s", "metadata")

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "[debug] loading metadata") {
		t.Fatalf("unexpected debug output: %s", output)
	}
}

func TestDebugf_Disabled(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := newTestCmd(false, false, false)
	Debugf(cmd, "should not appear")

	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.Len() > 0 {
		t.Fatalf("expected no output, got: %s", buf.String())
	}
}
```

Add `"bytes"`, `"strings"`, and `"fmt"` to the imports at the top of the test file.

- [ ] **Step 6: Run all cmdutil tests**

Run: `go test ./internal/cmdutil/ -v`
Expected: PASS (all 7 tests)

- [ ] **Step 7: Commit**

```bash
git add internal/cmdutil/output.go internal/cmdutil/output_test.go
git commit -m "feat: add cmdutil package with JSON, debug, and interactive mode helpers"
```

---

### Task 2: Wire global flags in `cmd/root.go` and handle JSON errors

**Files:**
- Modify: `cmd/root.go`
- Create: `cmd/root_flags_test.go`

- [ ] **Step 1: Write failing tests for global flags**

Create `cmd/root_flags_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
)

func TestGlobalFlags_Registered(t *testing.T) {
	// Verify flags exist on root command
	for _, name := range []string{"json", "debug", "no-interactive"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("expected persistent flag %q on root command", name)
		}
	}
}

func TestGlobalFlags_JSONMode(t *testing.T) {
	rootCmd.PersistentFlags().Set("json", "true")
	defer rootCmd.PersistentFlags().Set("json", "false")

	if !cmdutil.JSONMode(rootCmd) {
		t.Fatal("expected JSONMode true")
	}
}

func TestGlobalFlags_DebugMode(t *testing.T) {
	rootCmd.PersistentFlags().Set("debug", "true")
	defer rootCmd.PersistentFlags().Set("debug", "false")

	if !cmdutil.DebugMode(rootCmd) {
		t.Fatal("expected DebugMode true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -v -run 'TestGlobalFlags'`
Expected: FAIL — flags not registered yet.

- [ ] **Step 3: Add global flags and JSON error handling to root.go**

Modify `cmd/root.go`. The full file should become:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/spf13/cobra"
)

// Version information - injected at build time via ldflags
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "gs",
	Short: "gs - blazing fast git stack management",
	Long: `gs (git-stack) is a fast, simple git stack management tool.

It helps you work with stacked diffs (stacked PRs) efficiently,
maintaining parent-child relationships between branches.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmdutil.JSONMode(cmd) {
			colors.SetEnabled(false)
		}
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if cmdutil.JSONMode(rootCmd) {
			// Extract command name for the error response
			cmdName := ""
			if sub, _, _ := rootCmd.Find(os.Args[1:]); sub != nil && sub != rootCmd {
				cmdName = sub.Name()
			}
			cmdutil.PrintJSONError(cmdName, err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().Bool("json", false, "Output result as JSON to stdout")
	rootCmd.PersistentFlags().Bool("debug", false, "Print timestamped debug lines to stderr")
	rootCmd.PersistentFlags().Bool("no-interactive", false, "Disable interactive prompts")

	// Override default version template to show more info
	rootCmd.SetVersionTemplate(`gs version {{.Version}}
`)
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() string {
	return fmt.Sprintf("gs version %s\ncommit: %s\nbuilt: %s", Version, Commit, BuildDate)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -v -run 'TestGlobalFlags'`
Expected: PASS

- [ ] **Step 5: Run the full test suite to check for regressions**

Run: `go test ./...`
Expected: PASS — adding persistent flags should not break existing commands.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_flags_test.go
git commit -m "feat: wire --json, --debug, --no-interactive global flags on root command"
```

---

### Task 3: Convert `gs log` to JSON-first output

**Files:**
- Modify: `cmd/log.go`
- Create: `cmd/log_json_test.go`

This is the proof-of-concept command. `gs log` is ideal because it's read-only, already has a well-defined MCP JSON equivalent (`handleLog` in `internal/mcptools/read.go`), and its human output is complex enough to validate the pattern.

- [ ] **Step 1: Write failing test for `gs log --json`**

Create `cmd/log_json_test.go`:

```go
package cmd

import (
	"encoding/json"
	"testing"
)

// logResult mirrors the struct defined in log.go (tested via JSON round-trip)
func TestRunLog_JSON(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-a", "main")
	repo.createBranch(t, "feat-b", "feat-a")

	// Set --json flag
	rootCmd.PersistentFlags().Set("json", "true")
	defer rootCmd.PersistentFlags().Set("json", "false")

	// Capture stdout
	old := captureStdout(t)
	err := runLog(logCmd, nil)
	output := old.restore(t)

	if err != nil {
		t.Fatalf("runLog --json failed: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, output)
	}

	// Verify required fields
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

	// --short is ignored in JSON mode; output should still be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, output)
	}
}

// captureStdout helper — captures os.Stdout into a buffer.
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
```

Add `"bytes"` and `"os"` to the imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -v -run 'TestRunLog_JSON$' -count=1`
Expected: FAIL — `runLog` does not check `--json` yet.

- [ ] **Step 3: Define logResult struct and refactor runLog**

Modify `cmd/log.go` to the following:

```go
package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	logShort bool
	logLong  bool
)

// logResult is the JSON-first result struct for gs log.
type logResult struct {
	Trunk         string           `json:"trunk"`
	CurrentBranch string           `json:"current_branch"`
	Branches      []logBranchInfo  `json:"branches"`
}

type logBranchInfo struct {
	Name      string   `json:"name"`
	Parent    string   `json:"parent,omitempty"`
	Children  []string `json:"children"`
	CommitSHA string   `json:"commit_sha"`
	Depth     int      `json:"depth"`
	IsCurrent bool     `json:"is_current"`
	IsTrunk   bool     `json:"is_trunk"`
	PRURL     string   `json:"pr_url,omitempty"`
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display a visual representation of the current stack",
	Long: `Display a visual representation of the stack structure.

Shows branches as a tree starting from the trunk branch, with
the current branch highlighted.

Modes:
  gs log         - Standard tree view (*branch = current)
  gs log --short - Compact indented view (● = current, ○ = other)
  gs log --long  - Detailed view with commit messages
  gs log --json  - Machine-readable JSON output`,
	RunE: runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVar(&logShort, "short", false, "Show compact view")
	logCmd.Flags().BoolVar(&logLong, "long", false, "Show detailed view with commit messages")
}

func buildLogResult(rs *repoState) logResult {
	branches := make([]logBranchInfo, 0, len(rs.Stack.Nodes))

	// Trunk first
	if rs.Stack.Trunk != nil {
		branches = append(branches, nodeToLogBranch(rs.Stack, rs.Stack.Trunk))
	}

	// Non-trunk in topological order
	for _, node := range rs.Stack.GetTopologicalOrder() {
		branches = append(branches, nodeToLogBranch(rs.Stack, node))
	}

	return logResult{
		Trunk:         rs.Stack.TrunkName,
		CurrentBranch: rs.Stack.Current,
		Branches:      branches,
	}
}

func nodeToLogBranch(s *stack.Stack, node *stack.Node) logBranchInfo {
	children := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child.Name)
	}

	parentName := ""
	if node.Parent != nil {
		parentName = node.Parent.Name
	}

	sha := node.CommitSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}

	return logBranchInfo{
		Name:      node.Name,
		Parent:    parentName,
		Children:  children,
		CommitSHA: sha,
		Depth:     s.GetStackDepth(node.Name),
		IsCurrent: node.IsCurrent,
		IsTrunk:   node.IsTrunk,
		PRURL:     node.PRURL,
	}
}

func runLog(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	if err := rs.Stack.ValidateStack(); err != nil {
		return fmt.Errorf("invalid stack structure: %w", err)
	}

	// Populate PR URLs from remote (best-effort, non-fatal)
	if remoteURL, err := rs.Repo.GetRemoteURL("origin"); err == nil {
		if host, owner, repoName, err := provider.ParseRemoteURL(remoteURL); err == nil {
			rs.Stack.SetPRURLs(fmt.Sprintf("https://%s/%s/%s", host, owner, repoName))
		}
	}

	// JSON mode: return structured result
	if cmdutil.JSONMode(cmd) {
		return cmdutil.PrintJSON(buildLogResult(rs))
	}

	// Human mode: render tree as before
	var output string
	if logShort {
		output = rs.Stack.RenderShort(rs.Repo)
	} else {
		opts := stack.TreeOptions{
			ShowCommitSHA: true,
			ShowCommitMsg: logLong,
			Detailed:      logLong,
		}
		output = rs.Stack.RenderTree(rs.Repo, opts)
	}

	fmt.Print(output)
	return nil
}
```

- [ ] **Step 4: Run the JSON test to verify it passes**

Run: `go test ./cmd/ -v -run 'TestRunLog_JSON' -count=1`
Expected: PASS (both `TestRunLog_JSON` and `TestRunLog_JSON_Short_Ignored`)

- [ ] **Step 5: Run existing log tests to verify no regressions**

Run: `go test ./cmd/ -v -run 'Log' -count=1`
Expected: PASS — all existing log tests still pass since the human output path is unchanged.

- [ ] **Step 6: Run the full test suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/log.go cmd/log_json_test.go
git commit -m "feat: add --json support to gs log command (proof of concept)"
```

---

### Task 4: Manual verification and cleanup

- [ ] **Step 1: Build and test manually**

```bash
go build -o gs .
./gs log
./gs log --json
./gs log --json --short
./gs log --debug
./gs log --json --debug
```

Verify:
- `gs log` — same colored tree output as before
- `gs log --json` — valid JSON with `trunk`, `current_branch`, `branches[]`
- `gs log --json --short` — same JSON (--short ignored in JSON mode)
- `gs log --debug` — colored tree + debug lines on stderr
- `gs log --json --debug` — JSON on stdout, debug lines on stderr

- [ ] **Step 2: Test JSON error output**

```bash
cd /tmp && ./gs log --json 2>&1
```

Expected: JSON error on stderr with `{"error": "...", "command": "log"}` and non-zero exit code.

- [ ] **Step 3: Commit any fixes found during manual testing**

Only if needed. If everything passes, skip this step.

---

## Summary

After completing these 4 tasks, the codebase will have:

1. `internal/cmdutil/` — reusable helpers for JSON output, debug logging, and flag introspection
2. Global `--json`, `--debug`, `--no-interactive` flags on every command
3. `gs log` fully converted to JSON-first output as the proof-of-concept
4. A clear pattern for converting remaining commands (define result struct → `buildXResult()` → `if JSONMode → PrintJSON` → else human format)

Remaining commands can be converted one-at-a-time following the same pattern. MCP handler migration (switching from direct Go calls to `gs <cmd> --json` exec) is a separate follow-up.
