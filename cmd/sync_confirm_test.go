package cmd

import (
	"testing"
)

func withReadKey(key byte, fn func()) {
	prev := readKeyFn
	readKeyFn = func() (byte, error) { return key, nil }
	defer func() { readKeyFn = prev }()
	fn()
}

func withReadKeySequence(keys []byte, fn func()) {
	prev := readKeyFn
	idx := 0
	readKeyFn = func() (byte, error) {
		if idx >= len(keys) {
			return 'n', nil
		}
		b := keys[idx]
		idx++
		return b, nil
	}
	defer func() { readKeyFn = prev }()
	fn()
}

func TestSyncConfirmHelpers(t *testing.T) {
	tests := []struct {
		key    byte
		expect bool
	}{
		{'y', true},
		{'Y', true},
		{'n', false},
		{'x', false},
	}

	for _, tc := range tests {
		withReadKey(tc.key, func() {
			if got := confirm(); got != tc.expect {
				t.Fatalf("confirm(%c) = %v, want %v", tc.key, got, tc.expect)
			}
		})
	}
}

func TestSyncConfirmWithOptions(t *testing.T) {
	tests := []struct {
		key    byte
		expect string
	}{
		{'y', "yes"},
		{'Y', "yes"},
		{'a', "all"},
		{'A', "all"},
		{'q', "quit"},
		{'Q', "quit"},
		{'n', "no"},
		{'x', "no"},
	}

	for _, tc := range tests {
		withReadKey(tc.key, func() {
			if got := confirmWithOptions(); got != tc.expect {
				t.Fatalf("confirmWithOptions(%c) = %s, want %s", tc.key, got, tc.expect)
			}
		})
	}
}
