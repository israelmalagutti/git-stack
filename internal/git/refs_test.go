package git

import (
	"strings"
	"testing"
)

func TestEncodeBranchRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feat-auth", "feat-auth"},
		{"feat/auth", "feat--auth"},
		{"feat/auth/ui", "feat--auth--ui"},
		{"main", "main"},
		{"release/v1.0/hotfix", "release--v1.0--hotfix"},
	}

	for _, tt := range tests {
		got := EncodeBranchRef(tt.input)
		if got != tt.expected {
			t.Errorf("EncodeBranchRef(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDecodeBranchRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feat-auth", "feat-auth"},
		{"feat--auth", "feat/auth"},
		{"feat--auth--ui", "feat/auth/ui"},
		{"main", "main"},
	}

	for _, tt := range tests {
		got := DecodeBranchRef(tt.input)
		if got != tt.expected {
			t.Errorf("DecodeBranchRef(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	branches := []string{
		"main",
		"feat-auth",
		"feat/auth",
		"feat/auth/ui",
		"release/v1.0/hotfix",
	}

	for _, branch := range branches {
		encoded := EncodeBranchRef(branch)
		decoded := DecodeBranchRef(encoded)
		if decoded != branch {
			t.Errorf("round-trip failed for %q: encoded=%q decoded=%q", branch, encoded, decoded)
		}
	}
}

func TestValidateBranchForRefEncoding(t *testing.T) {
	t.Run("accepts normal branch names", func(t *testing.T) {
		valid := []string{"main", "feat-auth", "feat/auth", "release/v1.0/hotfix", "fix-123"}
		for _, name := range valid {
			if err := ValidateBranchForRefEncoding(name); err != nil {
				t.Errorf("expected %q to be valid, got error: %v", name, err)
			}
		}
	})

	t.Run("rejects branch names with double dash", func(t *testing.T) {
		invalid := []string{"feat--auth", "my--branch--name", "--leading", "trailing--"}
		for _, name := range invalid {
			if err := ValidateBranchForRefEncoding(name); err == nil {
				t.Errorf("expected %q to be rejected", name)
			}
		}
	})
}

func TestWriteAndReadRef(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("write and read round-trip", func(t *testing.T) {
		data := []byte(`{"parent":"main","created":"2026-03-27T14:30:00Z"}`)
		if err := repo.WriteRef("meta/feat-auth", data); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}

		got, err := repo.ReadRef("meta/feat-auth")
		if err != nil {
			t.Fatalf("ReadRef failed: %v", err)
		}

		if string(got) != string(data) {
			t.Errorf("expected %q, got %q", string(data), string(got))
		}
	})

	t.Run("overwrite existing ref", func(t *testing.T) {
		data1 := []byte(`{"parent":"main"}`)
		data2 := []byte(`{"parent":"feat-1"}`)

		if err := repo.WriteRef("meta/feat-overwrite", data1); err != nil {
			t.Fatalf("WriteRef first failed: %v", err)
		}
		if err := repo.WriteRef("meta/feat-overwrite", data2); err != nil {
			t.Fatalf("WriteRef second failed: %v", err)
		}

		got, err := repo.ReadRef("meta/feat-overwrite")
		if err != nil {
			t.Fatalf("ReadRef failed: %v", err)
		}

		if string(got) != string(data2) {
			t.Errorf("expected %q, got %q", string(data2), string(got))
		}
	})
}

func TestReadRefNotFound(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	_, err = repo.ReadRef("meta/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ref")
	}
}

func TestRefExists(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("returns false for nonexistent ref", func(t *testing.T) {
		if repo.RefExists("meta/nope") {
			t.Error("expected false for nonexistent ref")
		}
	})

	t.Run("returns true for existing ref", func(t *testing.T) {
		if err := repo.WriteRef("meta/exists", []byte(`{}`)); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}
		if !repo.RefExists("meta/exists") {
			t.Error("expected true for existing ref")
		}
	})
}

func TestDeleteRef(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("delete existing ref", func(t *testing.T) {
		data := []byte(`{"parent":"main"}`)
		if err := repo.WriteRef("meta/to-delete", data); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}

		if err := repo.DeleteRef("meta/to-delete"); err != nil {
			t.Fatalf("DeleteRef failed: %v", err)
		}

		_, err := repo.ReadRef("meta/to-delete")
		if err == nil {
			t.Error("expected error after deleting ref")
		}
	})

	t.Run("delete nonexistent ref returns error", func(t *testing.T) {
		err := repo.DeleteRef("meta/never-existed")
		if err == nil {
			t.Error("expected error for deleting nonexistent ref")
		}
	})
}

func TestListRefs(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("empty list when no refs", func(t *testing.T) {
		refs, err := repo.ListRefs("meta/")
		if err != nil {
			t.Fatalf("ListRefs failed: %v", err)
		}
		if len(refs) != 0 {
			t.Errorf("expected 0 refs, got %d: %v", len(refs), refs)
		}
	})

	t.Run("lists created refs", func(t *testing.T) {
		if err := repo.WriteRef("meta/feat-a", []byte(`{"parent":"main"}`)); err != nil {
			t.Fatalf("WriteRef feat-a failed: %v", err)
		}
		if err := repo.WriteRef("meta/feat-b", []byte(`{"parent":"feat-a"}`)); err != nil {
			t.Fatalf("WriteRef feat-b failed: %v", err)
		}
		if err := repo.WriteRef("config", []byte(`{"trunk":"main"}`)); err != nil {
			t.Fatalf("WriteRef config failed: %v", err)
		}

		// List only meta/ refs
		refs, err := repo.ListRefs("meta/")
		if err != nil {
			t.Fatalf("ListRefs meta/ failed: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("expected 2 meta refs, got %d: %v", len(refs), refs)
		}

		// Verify returned names are relative to refs/gs/
		found := map[string]bool{}
		for _, ref := range refs {
			found[ref] = true
		}
		if !found["meta/feat-a"] {
			t.Error("expected meta/feat-a in list")
		}
		if !found["meta/feat-b"] {
			t.Error("expected meta/feat-b in list")
		}
	})

	t.Run("list does not include other prefixes", func(t *testing.T) {
		refs, err := repo.ListRefs("meta/")
		if err != nil {
			t.Fatalf("ListRefs failed: %v", err)
		}
		for _, ref := range refs {
			if ref == "config" {
				t.Error("config ref should not appear in meta/ listing")
			}
		}
	})
}

func TestRefsWithEncodedSlashes(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Branch names with slashes get encoded so they don't conflict in git ref paths
	authRef := "meta/" + EncodeBranchRef("feat/auth")       // meta/feat--auth
	authUIRef := "meta/" + EncodeBranchRef("feat/auth/ui")   // meta/feat--auth--ui

	if err := repo.WriteRef(authRef, []byte(`{"parent":"main"}`)); err != nil {
		t.Fatalf("WriteRef feat/auth failed: %v", err)
	}
	if err := repo.WriteRef(authUIRef, []byte(`{"parent":"feat/auth"}`)); err != nil {
		t.Fatalf("WriteRef feat/auth/ui failed: %v", err)
	}

	refs, err := repo.ListRefs("meta/")
	if err != nil {
		t.Fatalf("ListRefs failed: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d: %v", len(refs), refs)
	}

	// Decode back to branch names
	branches := map[string]bool{}
	for _, ref := range refs {
		encoded := strings.TrimPrefix(ref, "meta/")
		branches[DecodeBranchRef(encoded)] = true
	}
	if !branches["feat/auth"] {
		t.Error("expected feat/auth in decoded branches")
	}
	if !branches["feat/auth/ui"] {
		t.Error("expected feat/auth/ui in decoded branches")
	}

	// Verify we can read back the data
	data, err := repo.ReadRef(authRef)
	if err != nil {
		t.Fatalf("ReadRef failed: %v", err)
	}
	if string(data) != `{"parent":"main"}` {
		t.Errorf("unexpected data: %s", string(data))
	}
}
