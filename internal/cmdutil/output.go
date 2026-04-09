package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// getBool looks up a boolean flag by name, traversing the command's flag chain
// including inherited persistent flags from parent commands.
func getBool(cmd *cobra.Command, name string) bool {
	f := cmd.Flag(name)
	if f == nil {
		return false
	}
	return f.Value.String() == "true"
}

// JSONMode returns true if the --json persistent flag is set.
func JSONMode(cmd *cobra.Command) bool {
	return getBool(cmd, "json")
}

// DebugMode returns true if the --debug persistent flag is set.
func DebugMode(cmd *cobra.Command) bool {
	return getBool(cmd, "debug")
}

// InteractiveMode returns true unless --json or --no-interactive is set.
func InteractiveMode(cmd *cobra.Command) bool {
	if JSONMode(cmd) {
		return false
	}
	return !getBool(cmd, "no-interactive")
}

// PrintJSON marshals data as indented JSON and writes it to stdout.
func PrintJSON(data any) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, string(out))
	return err
}

// PrintJSONError writes a JSON error object with "error" and "command" fields to stderr.
func PrintJSONError(command string, err error) {
	obj := map[string]string{
		"error":   err.Error(),
		"command": command,
	}
	out, _ := json.MarshalIndent(obj, "", "  ")
	fmt.Fprintln(os.Stderr, string(out))
}

// Debugf writes a timestamped debug line to stderr if debug mode is enabled.
func Debugf(cmd *cobra.Command, format string, args ...interface{}) {
	if !DebugMode(cmd) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(os.Stderr, "%s [debug] %s\n", ts, msg)
}
