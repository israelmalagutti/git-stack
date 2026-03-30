// Package colors provides terminal color utilities for CLI output.
// It uses ANSI 16 colors which adapt to the user's terminal theme.
package colors

import (
	"fmt"
	"os"
	"strings"
)

// ANSI escape codes for the 16-color palette (theme-adaptive)
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Italic = "\033[3m"

	// Standard colors (30-37)
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright/High-intensity colors (90-97)
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// enabled tracks whether color output is enabled
var enabled = true

func init() {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		enabled = false
		return
	}

	// Check for dumb terminal
	if os.Getenv("TERM") == "dumb" {
		enabled = false
		return
	}

	// Check if stdout is a terminal
	if fi, err := os.Stdout.Stat(); err == nil {
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			// Output is being piped, disable colors
			enabled = false
		}
	}
}

// SetEnabled allows manually enabling/disabling colors
func SetEnabled(e bool) {
	enabled = e
}

// IsEnabled returns whether colors are enabled
func IsEnabled() bool {
	return enabled
}

// apply wraps text with the given ANSI codes if colors are enabled
func apply(code, text string) string {
	if !enabled || text == "" {
		return text
	}
	return code + text + Reset
}

// applyMultiple wraps text with multiple ANSI codes
func applyMultiple(text string, codes ...string) string {
	if !enabled || text == "" {
		return text
	}
	return strings.Join(codes, "") + text + Reset
}

// Semantic color functions - these convey meaning

// Success formats text for successful operations (green)
func Success(text string) string {
	return apply(Green, text)
}

// Error formats text for errors (bright red)
func Error(text string) string {
	return apply(BrightRed, text)
}

// Warning formats text for warnings (yellow)
func Warning(text string) string {
	return apply(Yellow, text)
}

// Info formats text for informational messages (cyan)
func Info(text string) string {
	return apply(Cyan, text)
}

// Muted formats text for secondary/less important info (bright black/gray)
func Muted(text string) string {
	return apply(BrightBlack, text)
}

// Highlight formats text that should stand out (bright white + bold)
func Highlight(text string) string {
	return applyMultiple(text, Bold, BrightWhite)
}

// Branch color functions - for stack visualization

// BranchCurrent formats the current branch name (green + bold)
func BranchCurrent(text string) string {
	return applyMultiple(text, Bold, Green)
}

// BranchParent formats parent branch names (cyan)
func BranchParent(text string) string {
	return apply(Cyan, text)
}

// BranchChild formats child branch names (magenta)
func BranchChild(text string) string {
	return apply(Magenta, text)
}

// BranchTrunk formats the trunk branch name (blue + bold)
func BranchTrunk(text string) string {
	return applyMultiple(text, Bold, Blue)
}

// Status color functions - for PR/review status

// StatusApproved formats approved status (green)
func StatusApproved(text string) string {
	return apply(Green, text)
}

// StatusPending formats pending/review required status (yellow)
func StatusPending(text string) string {
	return apply(Yellow, text)
}

// StatusChangesRequested formats changes requested status (magenta)
func StatusChangesRequested(text string) string {
	return apply(Magenta, text)
}

// StatusDraft formats draft status (gray)
func StatusDraft(text string) string {
	return apply(BrightBlack, text)
}

// Text formatting functions

// BoldText makes text bold
func BoldText(text string) string {
	return apply(Bold, text)
}

// DimText makes text dim/faded
func DimText(text string) string {
	return apply(Dim, text)
}

// SubduedText applies dim + color for a desaturated but still colored look
func SubduedText(text string, index int) string {
	return applyMultiple(text, Dim, Cycle(index))
}

// ItalicText makes text italic (not supported in all terminals)
func ItalicText(text string) string {
	return apply(Italic, text)
}

// Cycling palette for stack tree visualization
// These use the base ANSI colors to remain theme-adaptive
var cyclePalette = []string{
	Cyan,
	Green,
	Yellow,
	Blue,
	Magenta,
	BrightCyan,
	BrightGreen,
	BrightYellow,
	BrightBlue,
}

// Cycle returns a color from the cycling palette based on index
func Cycle(index int) string {
	return cyclePalette[index%len(cyclePalette)]
}

// CycleText applies cycling color to text based on depth/index
func CycleText(text string, index int) string {
	return apply(Cycle(index), text)
}

// Sprintf is a colored version of fmt.Sprintf
func Sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}

// Tree drawing characters with colors

// TreeChars provides characters for tree visualization
type TreeChars struct {
	Vertical     string // │
	Horizontal   string // ─
	Corner       string // └
	TopCorner    string // ┌
	BottomRight  string // ┘
	Tee          string // ├
	Arrow        string // ▸ or >
	Bullet       string // ●
	Circle       string // ○
	FilledCircle string // ◉
}

// DefaultTreeChars returns the default tree drawing characters
func DefaultTreeChars() TreeChars {
	return TreeChars{
		Vertical:     "│",
		Horizontal:   "─",
		Corner:       "└",
		TopCorner:    "┌",
		BottomRight:  "┘",
		Tee:          "├",
		Arrow:        "▸",
		Bullet:       "●",
		Circle:       "○",
		FilledCircle: "◉",
	}
}

// ASCIITreeChars returns ASCII-only tree characters for limited terminals
func ASCIITreeChars() TreeChars {
	return TreeChars{
		Vertical:     "|",
		Horizontal:   "-",
		Corner:       "`",
		TopCorner:    ",",
		BottomRight:  "'",
		Tee:          "|",
		Arrow:        ">",
		Bullet:       "*",
		Circle:       "o",
		FilledCircle: "@",
	}
}

// CommitSHA formats a commit SHA (yellow/dim)
func CommitSHA(text string) string {
	return apply(Yellow, text)
}

// Hyperlink wraps text in an OSC 8 hyperlink escape sequence, making it
// clickable in supported terminals (iTerm2, Windows Terminal, GNOME Terminal,
// Konsole, etc.). Terminals that don't support OSC 8 silently ignore the
// escape and display the text as-is.
func Hyperlink(url, text string) string {
	if !enabled || url == "" {
		return text
	}
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}
