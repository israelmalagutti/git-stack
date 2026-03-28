package git

import (
	"os"
	"testing"
)

// saveAndRestoreCwd saves the current directory and returns a cleanup function.
func saveAndRestoreCwd(t *testing.T) func() {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	return func() {
		_ = os.Chdir(origDir)
	}
}

func TestHasRemote(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("returns true for existing remote", func(t *testing.T) {
		if !repo.HasRemote("origin") {
			t.Error("expected origin to exist")
		}
	})

	t.Run("returns false for nonexistent remote", func(t *testing.T) {
		if repo.HasRemote("upstream") {
			t.Error("expected upstream to not exist")
		}
	})
}

func TestConfigureRefspec(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	refspec := "+refs/gs/*:refs/gs/*"

	t.Run("adds refspec when not present", func(t *testing.T) {
		has, err := repo.HasRefspec("origin", refspec)
		if err != nil {
			t.Fatalf("HasRefspec failed: %v", err)
		}
		if has {
			t.Fatal("expected refspec to not exist yet")
		}

		if err := repo.ConfigureRefspec("origin", refspec); err != nil {
			t.Fatalf("ConfigureRefspec failed: %v", err)
		}

		has, err = repo.HasRefspec("origin", refspec)
		if err != nil {
			t.Fatalf("HasRefspec failed: %v", err)
		}
		if !has {
			t.Error("expected refspec to exist after configure")
		}
	})

	t.Run("idempotent - does not duplicate", func(t *testing.T) {
		if err := repo.ConfigureRefspec("origin", refspec); err != nil {
			t.Fatalf("ConfigureRefspec second call failed: %v", err)
		}

		output, err := repo.RunGitCommand("config", "--get-all", "remote.origin.fetch")
		if err != nil {
			t.Fatalf("git config failed: %v", err)
		}

		count := 0
		for _, line := range refTestSplitLines(output) {
			if line == refspec {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected refspec to appear once, got %d", count)
		}
	})
}

func TestPushAndFetchRefs(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("push refs to remote and fetch back", func(t *testing.T) {
		data := []byte(`{"parent":"main","created":"2026-03-27T14:30:00Z"}`)
		if err := repo.WriteRef("meta/feat-auth", data); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}

		if err := repo.PushRefs("origin", "refs/gs/meta/feat-auth:refs/gs/meta/feat-auth"); err != nil {
			t.Fatalf("PushRefs failed: %v", err)
		}

		if err := repo.DeleteRef("meta/feat-auth"); err != nil {
			t.Fatalf("DeleteRef failed: %v", err)
		}

		if repo.RefExists("meta/feat-auth") {
			t.Fatal("expected ref to be deleted locally")
		}

		if err := repo.FetchRefs("origin", "+refs/gs/*:refs/gs/*"); err != nil {
			t.Fatalf("FetchRefs failed: %v", err)
		}

		if !repo.RefExists("meta/feat-auth") {
			t.Error("expected ref to exist after fetch")
		}

		got, err := repo.ReadRef("meta/feat-auth")
		if err != nil {
			t.Fatalf("ReadRef failed: %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("expected %q, got %q", string(data), string(got))
		}
	})
}

func TestPushAllAndFetchAll(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	if err := repo.WriteRef("config", []byte(`{"trunk":"main"}`)); err != nil {
		t.Fatalf("WriteRef config failed: %v", err)
	}
	if err := repo.WriteRef("meta/feat-a", []byte(`{"parent":"main"}`)); err != nil {
		t.Fatalf("WriteRef feat-a failed: %v", err)
	}
	if err := repo.WriteRef("meta/feat-b", []byte(`{"parent":"feat-a"}`)); err != nil {
		t.Fatalf("WriteRef feat-b failed: %v", err)
	}

	if err := repo.PushRefs("origin", "refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("PushRefs all failed: %v", err)
	}

	for _, ref := range []string{"config", "meta/feat-a", "meta/feat-b"} {
		if err := repo.DeleteRef(ref); err != nil {
			t.Fatalf("DeleteRef %s failed: %v", ref, err)
		}
	}

	if err := repo.FetchRefs("origin", "+refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("FetchRefs all failed: %v", err)
	}

	for _, ref := range []string{"config", "meta/feat-a", "meta/feat-b"} {
		if !repo.RefExists(ref) {
			t.Errorf("expected ref %s to exist after fetch", ref)
		}
	}
}

func TestDeleteRemoteRef(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	if err := repo.WriteRef("meta/to-delete", []byte(`{"parent":"main"}`)); err != nil {
		t.Fatalf("WriteRef failed: %v", err)
	}
	if err := repo.PushRefs("origin", "refs/gs/meta/to-delete:refs/gs/meta/to-delete"); err != nil {
		t.Fatalf("PushRefs failed: %v", err)
	}

	if err := repo.DeleteRemoteRef("origin", "meta/to-delete"); err != nil {
		t.Fatalf("DeleteRemoteRef failed: %v", err)
	}

	if err := repo.DeleteRef("meta/to-delete"); err != nil {
		t.Fatalf("DeleteRef local failed: %v", err)
	}

	_ = repo.FetchRefs("origin", "+refs/gs/*:refs/gs/*")

	if repo.RefExists("meta/to-delete") {
		t.Error("expected remote ref to be deleted")
	}
}

func TestCrossRepoRefSync(t *testing.T) {
	restoreCwd := saveAndRestoreCwd(t)
	defer restoreCwd()

	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	// Alice: write refs and push
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir to local: %v", err)
	}

	alice, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (alice) failed: %v", err)
	}

	if err := alice.WriteRef("config", []byte(`{"trunk":"main","version":"1.0.0"}`)); err != nil {
		t.Fatalf("WriteRef config failed: %v", err)
	}
	if err := alice.WriteRef("meta/feat--auth", []byte(`{"parent":"main"}`)); err != nil {
		t.Fatalf("WriteRef feat/auth failed: %v", err)
	}
	if err := alice.WriteRef("meta/feat--auth-ui", []byte(`{"parent":"feat/auth"}`)); err != nil {
		t.Fatalf("WriteRef feat/auth-ui failed: %v", err)
	}

	if err := alice.PushRefs("origin", "refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("PushRefs (alice) failed: %v", err)
	}

	// Bob fetches from the "other" repo
	otherDir := localDir[:len(localDir)-len("local")] + "other"
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}

	bob, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (bob) failed: %v", err)
	}

	if err := bob.FetchRefs("origin", "+refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("FetchRefs (bob) failed: %v", err)
	}

	if !bob.RefExists("config") {
		t.Error("bob should have config ref")
	}
	if !bob.RefExists("meta/feat--auth") {
		t.Error("bob should have feat/auth ref")
	}
	if !bob.RefExists("meta/feat--auth-ui") {
		t.Error("bob should have feat/auth-ui ref")
	}

	configData, err := bob.ReadRef("config")
	if err != nil {
		t.Fatalf("ReadRef config (bob) failed: %v", err)
	}
	if string(configData) != `{"trunk":"main","version":"1.0.0"}` {
		t.Errorf("unexpected config data: %s", string(configData))
	}

	authData, err := bob.ReadRef("meta/feat--auth")
	if err != nil {
		t.Fatalf("ReadRef feat/auth (bob) failed: %v", err)
	}
	if string(authData) != `{"parent":"main"}` {
		t.Errorf("unexpected feat/auth data: %s", string(authData))
	}
}

func refTestSplitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			line := s[start:i]
			start = i + 1
			trimmed := refTestTrim(line)
			if trimmed != "" {
				lines = append(lines, trimmed)
			}
		}
	}
	if start < len(s) {
		trimmed := refTestTrim(s[start:])
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func refTestTrim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
