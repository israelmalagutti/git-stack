#!/usr/bin/env bash
set -euo pipefail

# Setup a complex branch structure for testing gw log, gw checkout, etc.
# Creates a git+gw repo in ./tmp/ with stacked branches and restack scenarios.
#
# Usage: ./scripts/setup-test-repo.sh
#   Run from the project root. Creates ./tmp/ with the test repo.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET="$PROJECT_ROOT/tmp"

if [ -d "$TARGET" ]; then
  echo "Removing existing test repo at $TARGET"
  rm -rf "$TARGET"
fi

mkdir -p "$TARGET"
cd "$TARGET"
git init
git checkout -b main

# Helper: create a commit that touches a specific file
commit() {
  local file="$1"
  local content="$2"
  local msg="$3"
  echo "$content" > "$file"
  git add "$file"
  git commit -m "$msg"
}

# ─── Trunk (main) ───────────────────────────────────────────────────────────
commit "README.md" "# Test Project" "initial commit"
commit "shared.txt" "line 1: original from main" "add shared.txt"

# ─── Initialize gw ──────────────────────────────────────────────────────────
# Create .gw_config and .gw_stack_metadata in .git/ (same as `gw init`)
cat > .git/.gw_config <<'GWCONFIG'
{
  "version": "1.0.0",
  "trunk": "main",
  "initialized": "2026-03-11T00:00:00Z"
}
GWCONFIG

# We'll build the metadata JSON at the end after all branches exist.
# For now, create an empty one so gw commands don't fail mid-script.
echo '{"branches":{}}' > .git/.gw_stack_metadata

# ─── Level 1: three direct children of main ─────────────────────────────────

# --- TEST_1 (3 children) ---
git checkout -b TEST_1 main
commit "test1.txt" "feature 1 work" "TEST_1: add feature 1"

git checkout -b TEST_1_1 TEST_1
commit "test1_1.txt" "sub-feature 1_1" "TEST_1_1: add sub-feature"

git checkout -b TEST_1_2 TEST_1
commit "test1_2.txt" "sub-feature 1_2" "TEST_1_2: add sub-feature"

git checkout -b TEST_1_3 TEST_1
commit "test1_3.txt" "sub-feature 1_3" "TEST_1_3: add sub-feature"

# --- TEST_2 (5 children, restack scenarios) ---
git checkout -b TEST_2 main
commit "test2.txt" "feature 2 work" "TEST_2: add feature 2"
# This file will be modified later to create restack conflicts
commit "shared_test2.txt" "line 1: original from TEST_2" "TEST_2: add shared_test2.txt"

git checkout -b TEST_2_1 TEST_2
commit "test2_1.txt" "sub-feature 2_1" "TEST_2_1: add sub-feature"
# Touch the same file to create a conflict with TEST_2's future changes
commit "shared_test2.txt" "line 1: modified by TEST_2_1" "TEST_2_1: modify shared_test2.txt"

git checkout -b TEST_2_2 TEST_2_1
commit "test2_2.txt" "sub-feature 2_2" "TEST_2_2: add sub-feature"

# Deep nesting: TEST_2_1 -> TEST_2_1_1 -> TEST_2_1_1_1
git checkout -b TEST_2_1_1 TEST_2_1
commit "test2_1_1.txt" "deep feature 2_1_1" "TEST_2_1_1: add deep sub-feature"

git checkout -b TEST_2_1_1_1 TEST_2_1_1
commit "test2_1_1_1.txt" "deepest feature 2_1_1_1" "TEST_2_1_1_1: add deepest sub-feature"

git checkout -b TEST_2_3 TEST_2
commit "test2_3.txt" "sub-feature 2_3" "TEST_2_3: add sub-feature"

git checkout -b TEST_2_4 TEST_2
commit "test2_4.txt" "sub-feature 2_4" "TEST_2_4: add sub-feature"

git checkout -b TEST_2_5 TEST_2
commit "test2_5.txt" "sub-feature 2_5" "TEST_2_5: add sub-feature"

# --- TEST_3 (3 children) ---
git checkout -b TEST_3 main
commit "test3.txt" "feature 3 work" "TEST_3: add feature 3"

git checkout -b TEST_3_1 TEST_3
commit "test3_1.txt" "sub-feature 3_1" "TEST_3_1: add sub-feature"

git checkout -b TEST_3_2 TEST_3
commit "test3_2.txt" "sub-feature 3_2" "TEST_3_2: add sub-feature"

git checkout -b TEST_3_3 TEST_3
commit "test3_3.txt" "sub-feature 3_3" "TEST_3_3: add sub-feature"

# ─── Create restack scenarios ────────────────────────────────────────────────

# Scenario 1: TEST_2 diverges from its children
# Go back to TEST_2 and add a conflicting commit AFTER children were created
git checkout TEST_2
commit "shared_test2.txt" "line 1: AMENDED by TEST_2 (conflict)" \
  "TEST_2: amend shared_test2.txt (will conflict with TEST_2_1)"

# Now TEST_2_1 (and TEST_2_2 which stacks on it) need restacking onto TEST_2.
# TEST_2_1_1 and TEST_2_1_1_1 also need restacking transitively.

# Scenario 2: main diverges from its direct children (trunk restack)
git checkout main
commit "shared.txt" "line 1: AMENDED by main (diverged)" \
  "main: amend shared.txt (trunk divergence for restack)"

# Now TEST_1, TEST_2, TEST_3 are all behind main and need restacking.

# ─── Write gw stack metadata (track all branches) ───────────────────────────
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cat > .git/.gw_stack_metadata <<EOF
{
  "branches": {
    "TEST_1":       {"parent": "main",       "tracked": true, "created": "$NOW"},
    "TEST_1_1":     {"parent": "TEST_1",     "tracked": true, "created": "$NOW"},
    "TEST_1_2":     {"parent": "TEST_1",     "tracked": true, "created": "$NOW"},
    "TEST_1_3":     {"parent": "TEST_1",     "tracked": true, "created": "$NOW"},
    "TEST_2":       {"parent": "main",       "tracked": true, "created": "$NOW"},
    "TEST_2_1":     {"parent": "TEST_2",     "tracked": true, "created": "$NOW"},
    "TEST_2_2":     {"parent": "TEST_2_1",   "tracked": true, "created": "$NOW"},
    "TEST_2_1_1":   {"parent": "TEST_2_1",   "tracked": true, "created": "$NOW"},
    "TEST_2_1_1_1": {"parent": "TEST_2_1_1", "tracked": true, "created": "$NOW"},
    "TEST_2_3":     {"parent": "TEST_2",     "tracked": true, "created": "$NOW"},
    "TEST_2_4":     {"parent": "TEST_2",     "tracked": true, "created": "$NOW"},
    "TEST_2_5":     {"parent": "TEST_2",     "tracked": true, "created": "$NOW"},
    "TEST_3":       {"parent": "main",       "tracked": true, "created": "$NOW"},
    "TEST_3_1":     {"parent": "TEST_3",     "tracked": true, "created": "$NOW"},
    "TEST_3_2":     {"parent": "TEST_3",     "tracked": true, "created": "$NOW"},
    "TEST_3_3":     {"parent": "TEST_3",     "tracked": true, "created": "$NOW"}
  }
}
EOF

# ─── Done ────────────────────────────────────────────────────────────────────
git checkout main

echo ""
echo "=== Test repo created at: $TARGET ==="
echo ""
echo "Branch structure:"
echo "  main (trunk)"
echo "  ├── TEST_1"
echo "  │   ├── TEST_1_1"
echo "  │   ├── TEST_1_2"
echo "  │   └── TEST_1_3"
echo "  ├── TEST_2"
echo "  │   ├── TEST_2_1  ← needs restack (conflicts with TEST_2)"
echo "  │   │   ├── TEST_2_2  ← stacked on TEST_2_1"
echo "  │   │   ├── TEST_2_1_1"
echo "  │   │   │   └── TEST_2_1_1_1"
echo "  │   ├── TEST_2_3"
echo "  │   ├── TEST_2_4"
echo "  │   └── TEST_2_5"
echo "  └── TEST_3"
echo "      ├── TEST_3_1"
echo "      ├── TEST_3_2"
echo "      └── TEST_3_3"
echo ""
echo "Restack scenarios:"
echo "  1. TEST_2 diverged from TEST_2_1 (shared_test2.txt conflict)"
echo "     → TEST_2_1, TEST_2_2, TEST_2_1_1, TEST_2_1_1_1 need restacking"
echo "  2. main diverged from all level-1 branches (shared.txt changed)"
echo "     → TEST_1, TEST_2, TEST_3 need restacking from trunk"
echo ""
echo "To test: cd $TARGET && gw log"
