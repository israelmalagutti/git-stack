package cmd

import (
	"testing"
)

// --- log.go ---

func TestRunLogShort(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-log", "main")
	tr.commitFile(t, "log.txt", "data", "log commit")

	prev := logShort
	logShort = true
	defer func() { logShort = prev }()

	if err := runLog(nil, nil); err != nil {
		t.Fatalf("runLog short failed: %v", err)
	}
}

func TestRunLogDefault(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-log2", "main")
	tr.commitFile(t, "log2.txt", "data", "log commit 2")

	prevS := logShort
	prevL := logLong
	logShort = false
	logLong = false
	defer func() { logShort = prevS; logLong = prevL }()

	if err := runLog(nil, nil); err != nil {
		t.Fatalf("runLog default failed: %v", err)
	}
}

// --- children.go ---

func TestRunChildren_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-parent-c", "main")
	tr.createBranch(t, "feat-child-c", "feat-parent-c")

	if err := tr.repo.CheckoutBranch("feat-parent-c"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if err := runChildren(nil, nil); err != nil {
		t.Fatalf("runChildren failed: %v", err)
	}
}

func TestRunChildren_NoChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-leaf-c", "main")

	if err := runChildren(nil, nil); err != nil {
		t.Fatalf("runChildren no children failed: %v", err)
	}
}

// --- parent.go ---

func TestRunParent_FromBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-par", "main")

	if err := runParent(nil, nil); err != nil {
		t.Fatalf("runParent failed: %v", err)
	}
}

func TestRunParent_FromTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// On main (trunk)
	if err := runParent(nil, nil); err == nil || err.Error() != "trunk has no parent" {
		// May be "branch 'main' has no parent" or similar
		if err == nil {
			t.Fatal("expected error for trunk parent")
		}
	}
}

// --- info.go ---

func TestRunInfo_Branch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-info", "main")
	tr.commitFile(t, "info.txt", "data", "info commit")

	if err := runInfo(nil, []string{"feat-info"}); err != nil {
		t.Fatalf("runInfo branch failed: %v", err)
	}
}

func TestRunInfo_Current(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-info-cur", "main")

	if err := runInfo(nil, nil); err != nil {
		t.Fatalf("runInfo current failed: %v", err)
	}
}

func TestRunInfo_Trunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := runInfo(nil, []string{"main"}); err != nil {
		t.Fatalf("runInfo trunk failed: %v", err)
	}
}

func TestRunInfo_MissingBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := runInfo(nil, []string{"nonexistent"}); err == nil {
		t.Fatal("expected error for missing branch")
	}
}

// --- rename.go ---

func TestRunRename_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	if err := runRename(nil, []string{"old", "new"}); err == nil {
		t.Fatal("expected error without config")
	}
}

// --- delete.go ---

func TestRunDelete_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	if err := runDelete(nil, []string{"missing"}); err == nil {
		t.Fatal("expected error without config")
	}
}
