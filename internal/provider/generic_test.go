package provider

import (
	"errors"
	"testing"
)

func TestGenericProvider(t *testing.T) {
	g := NewGenericProvider("git.internal.com")

	t.Run("name", func(t *testing.T) {
		if g.Name() != "generic" {
			t.Errorf("expected 'generic', got %q", g.Name())
		}
	})

	t.Run("CLI not available", func(t *testing.T) {
		if g.CLIAvailable() {
			t.Error("expected false")
		}
	})

	t.Run("CLI not authenticated", func(t *testing.T) {
		if g.CLIAuthenticated() {
			t.Error("expected false")
		}
	})

	t.Run("CreatePR returns ErrNotSupported", func(t *testing.T) {
		_, err := g.CreatePR(PRCreateOpts{Base: "main", Head: "feat/auth"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})

	t.Run("UpdatePR returns ErrNotSupported", func(t *testing.T) {
		err := g.UpdatePR(1, PRUpdateOpts{})
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})

	t.Run("GetPRStatus returns ErrNotSupported", func(t *testing.T) {
		_, err := g.GetPRStatus(1)
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})

	t.Run("MergePR returns ErrNotSupported", func(t *testing.T) {
		err := g.MergePR(1, PRMergeOpts{})
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})

	t.Run("UpdatePRBase returns ErrNotSupported", func(t *testing.T) {
		err := g.UpdatePRBase(1, "main")
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})

	t.Run("FindExistingPR returns ErrNotSupported", func(t *testing.T) {
		result, err := g.FindExistingPR("feat/auth")
		if result != nil {
			t.Errorf("expected nil result, got: %v", result)
		}
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
	})
}
