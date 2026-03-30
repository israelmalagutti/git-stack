package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestRunRepair_NoIssues(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	repairDryRun = false
	repairForce = true
	defer func() {
		repairDryRun = false
		repairForce = false
	}()

	err := runRepair(repairCmd, nil)
	if err != nil {
		t.Fatalf("runRepair failed: %v", err)
	}
}

func TestRunRepair_DryRun(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a branch, track it, then delete the git branch to create an orphan
	tr.createBranch(t, "stale", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := tr.repo.DeleteBranch("stale", true); err != nil {
		t.Fatalf("delete stale: %v", err)
	}

	// Save metadata with the orphan ref
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	repairDryRun = true
	repairForce = false
	defer func() { repairDryRun = false }()

	err := runRepair(repairCmd, nil)
	if err != nil {
		t.Fatalf("runRepair dry-run failed: %v", err)
	}

	// Branch should still be in metadata (dry run = no changes)
	if !tr.metadata.IsTracked("stale") {
		t.Error("expected stale to still be tracked in dry run")
	}
}

func TestRunRepair_ForceFixOrphaned(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create an orphaned metadata entry
	tr.createBranch(t, "orphaned", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := tr.repo.DeleteBranch("orphaned", true); err != nil {
		t.Fatalf("delete orphaned: %v", err)
	}
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	repairDryRun = false
	repairForce = true
	defer func() {
		repairDryRun = false
		repairForce = false
	}()

	err := runRepair(repairCmd, nil)
	if err != nil {
		t.Fatalf("runRepair force failed: %v", err)
	}

	// Reload metadata and check
	meta, err := loadMetadata(tr.repo)
	if err != nil {
		t.Fatalf("loadMetadata: %v", err)
	}
	if meta.IsTracked("orphaned") {
		t.Error("expected orphaned to be untracked after force repair")
	}
}

func TestRunRepair_InteractiveConfirm(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create orphaned entry
	tr.createBranch(t, "fix-me", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := tr.repo.DeleteBranch("fix-me", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	repairDryRun = false
	repairForce = false
	defer func() { repairDryRun = false; repairForce = false }()

	// User confirms the fix
	withAskOne(t, []interface{}{true}, func() {
		err := runRepair(repairCmd, nil)
		if err != nil {
			t.Fatalf("runRepair interactive failed: %v", err)
		}
	})
}

func TestRunRepair_InteractiveDecline(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "skip-me", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := tr.repo.DeleteBranch("skip-me", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	repairDryRun = false
	repairForce = false
	defer func() { repairDryRun = false; repairForce = false }()

	// User declines
	withAskOne(t, []interface{}{false}, func() {
		err := runRepair(repairCmd, nil)
		if err != nil {
			t.Fatalf("runRepair decline failed: %v", err)
		}
	})

	// Should still be tracked since user declined
	if !tr.metadata.IsTracked("skip-me") {
		t.Error("expected skip-me to still be tracked after declining")
	}
}

func TestRunRepair_InteractiveInterrupt(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "int-me", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := tr.repo.DeleteBranch("int-me", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	repairDryRun = false
	repairForce = false
	defer func() { repairDryRun = false; repairForce = false }()

	// Simulate interrupt
	withAskOneError(t, terminal.InterruptErr, func() {
		err := runRepair(repairCmd, nil)
		if err != nil {
			t.Fatalf("runRepair interrupt should not error: %v", err)
		}
	})
}
