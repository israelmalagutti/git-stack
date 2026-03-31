package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// templateRepoDir holds a pre-initialized git repo that setupCmdTestRepo
// copies instead of running git init + config + commit from scratch.
// This avoids ~7 git subprocess spawns per test (~4000 total across the suite).
var templateRepoDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gs-template-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create template dir: %v\n", err)
		os.Exit(1)
	}

	// Build the template repo once
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "template setup %v: %v\n", args, err)
			os.Exit(1)
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write template README: %v\n", err)
		os.Exit(1)
	}

	addAndCommit := [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "Initial commit"},
		{"git", "-C", dir, "branch", "-M", "main"},
	}
	for _, args := range addAndCommit {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "template setup %v: %v\n", args, err)
			os.Exit(1)
		}
	}

	templateRepoDir = dir

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// copyDir recursively copies src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
