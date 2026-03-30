package cmd

import (
	"fmt"
	"testing"
)

func TestReadKeyFn_Error(t *testing.T) {
	prev := readKeyFn
	readKeyFn = func() (byte, error) {
		return 0, fmt.Errorf("read error")
	}
	defer func() { readKeyFn = prev }()

	// confirm should return false on error
	if confirm() {
		t.Error("expected false on read error")
	}

	// confirmWithOptions should return "no" on error
	if got := confirmWithOptions(); got != "no" {
		t.Errorf("expected 'no' on error, got %q", got)
	}
}

func TestReadKeyFn_AllOptions(t *testing.T) {
	tests := []struct {
		input byte
		want  string
	}{
		{'y', "yes"},
		{'Y', "yes"},
		{'n', "no"},
		{'N', "no"},
		{'a', "all"},
		{'A', "all"},
		{'q', "quit"},
		{'Q', "quit"},
	}

	for _, tt := range tests {
		prev := readKeyFn
		readKeyFn = func() (byte, error) {
			return tt.input, nil
		}

		got := confirmWithOptions()
		readKeyFn = prev

		if got != tt.want {
			t.Errorf("input=%c: expected %q, got %q", tt.input, tt.want, got)
		}
	}
}

func TestConfirmYesNo(t *testing.T) {
	// Test confirm returns true for 'y'
	prev := readKeyFn
	readKeyFn = func() (byte, error) { return 'y', nil }
	if !confirm() {
		t.Error("expected true for 'y'")
	}

	readKeyFn = func() (byte, error) { return 'Y', nil }
	if !confirm() {
		t.Error("expected true for 'Y'")
	}

	readKeyFn = func() (byte, error) { return 'n', nil }
	if confirm() {
		t.Error("expected false for 'n'")
	}
	readKeyFn = prev
}
