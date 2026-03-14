# Git Stack (gs) - Usage Guide

## Interactive Mode Commands

Many `gs` commands use interactive prompts to help you select branches, enter names, or make decisions. Here are the keyboard shortcuts available in interactive mode:

### Navigation & Selection
- **Arrow Keys (↑/↓)** - Navigate up and down through options
- **Tab** - Move to next option (in Select prompts)
- **Enter** - Confirm selection or submit input
- **Type to filter** - Start typing to filter options (in Select prompts)

### Vim Mode
- **ESC** - Toggle vim mode on/off
- **j** - Move down (when vim mode is enabled)
- **k** - Move up (when vim mode is enabled)

### Editing (Input prompts)
- **Backspace/Delete** - Remove characters
- **Ctrl+W** - Delete word
- **Ctrl+U** - Delete line

### Control
- **Ctrl+C** - Cancel and exit the current prompt

## Command Reference

### Initialization

#### `gs init`
Initialize git-stack in your repository. This sets up the configuration and identifies your trunk branch (usually `main` or `master`).

```bash
gs init
```

### Branch Management

#### `gs create [name]`
Create a new branch stacked on top of the current branch. The new branch is automatically tracked with the current branch as its parent.

```bash
# Interactive mode - prompts for branch name
gs create

# Direct mode - specify branch name
gs create feat-auth
```

If you have staged changes, you'll be prompted to commit them to the new branch.

#### `gs track`
Start tracking an existing branch. You'll be prompted to select a parent branch from your stack.

```bash
gs track
```

#### `gs checkout [options]`
Smart branch checkout with interactive selection. Shows stack context for each branch.

```bash
# Interactive mode - select from list of branches
gs co

# Quick checkout to trunk
gs co -t
gs co --trunk

# Show untracked branches in selection
gs co -u
gs co --show-untracked

# Only show branches in current stack
gs co -s
gs co --stack
```

**Aliases:** `co`, `checkout`, `switch`

### Visualization

#### `gs log [options]`
Display a visual tree representation of your branch stack.

```bash
# Compact view
gs log
gs log --short

# Detailed view with commit messages
gs log --long
```

Output format:
```
● *main (trunk) [fe9d15f]
├── feat-1 [a1cb412]
└── feat-2 [a1cb412]
```

Legend:
- `●` - Current branch (filled circle)
- `○` - Other branches (hollow circle)
- `*` - Indicator for current branch name
- `[hash]` - Commit SHA

#### `gs info`
Show detailed information about the current branch, including parent, children, depth in stack, and path to trunk.

```bash
gs info
```

#### `gs parent`
Show the parent branch of the current branch.

```bash
gs parent
```

#### `gs children`
Show all child branches of the current branch.

```bash
gs children
```

### Stack Maintenance

#### `gs stack restack`
Ensure each branch in the current stack is based on its parent, rebasing if necessary. This command recursively restacks all children branches.

```bash
# Restack current branch and all its children
gs stack restack

# Short aliases
gs stack r
gs stack fix
gs stack f
```

**What it does:**
- Checks if the current branch needs rebasing onto its parent
- Performs the rebase if the parent has moved forward
- Recursively restacks all children branches
- Handles conflicts interactively (prompts you to resolve and continue)

**When to use:**
- After making changes to a parent branch
- When trunk has moved forward and you want to update your stack
- To fix "out of sync" branches in your stack

**Aliases:** `r`, `fix`, `f`

#### `gs sync`
Clean up metadata and validate stack structure.

```bash
# Interactive cleanup
gs sync

# Force cleanup without prompts
gs sync -f
```

**What it does:**
- Removes metadata for branches that no longer exist in git
- Validates trunk branch has no parent
- Detects cycles in branch relationships
- Ensures stack structure is valid

#### `gs modify`
Modify the current branch by amending its commit or creating a new commit. Automatically restacks descendants.

```bash
# Amend current commit
gs modify

# Amend with message
gs modify -m "Updated commit message"

# Stage all changes and amend
gs modify -a

# Stage interactively and amend
gs modify -p

# Create new commit instead of amending
gs modify -c -m "New commit message"

# Short alias
gs m -a
```

**What it does:**
- Stages changes if requested (with `-a` or `-p`)
- Amends the current commit (default) or creates a new commit (with `-c`)
- Automatically restacks all children branches after the change
- Handles conflicts during restacking interactively

**Flags:**
- `-a, --all` - Stage all changes before committing
- `-p, --patch` - Interactively stage changes (prompts for each hunk)
- `-c, --commit` - Create a new commit instead of amending
- `-m, --message` - Specify commit message

**When to use:**
- When you want to make changes to the current branch's commit
- After code review feedback on a stacked branch
- To add forgotten changes to the current commit
- To split changes into a new commit

**Alias:** `m`

### Advanced Stack Operations

#### `gs move [target]`
Rebase the current branch onto a different parent branch. Automatically restacks all descendants.

```bash
# Interactive selection of target branch
gs move

# Move current branch onto feat-base
gs move feat-base

# Using --onto flag
gs move -o feat-base

# Short alias
gs mv feat-base
```

**What it does:**
- Changes the parent of the current branch
- Rebases the current branch onto the new parent
- Automatically restacks all children branches
- Handles conflicts interactively
- Prevents circular dependencies (won't move onto descendants)

**Flags:**
- `-o, --onto` - Specify target branch

**When to use:**
- Reorganizing your stack structure
- Moving a feature branch to depend on a different parent
- Extracting a branch from one stack to another
- Fixing incorrect parent-child relationships

**Alias:** `mv`

#### `gs fold`
Fold the current branch's changes into its parent branch. This merges the current branch into its parent and deletes it (unless --keep is used).

```bash
# Fold current branch into parent
gs fold

# Keep current branch instead of deleting it
gs fold --keep

# Skip confirmation prompt
gs fold --force
gs fold -f
```

**What it does:**
- Merges current branch's commits into parent (squash merge)
- Updates all children to point to the parent
- Restacks all descendants
- Deletes the current branch (unless --keep is used)
- Prompts for confirmation before proceeding

**Flags:**
- `-k, --keep` - Keep current branch name instead of deleting it
- `-f, --force` - Skip confirmation prompt

**When to use:**
- When you realize a branch should have been part of its parent
- Simplifying a stack by collapsing intermediate branches
- Cleaning up unnecessary branches after code review changes
- Combining multiple small changes into a single branch

#### `gs delete [branch]`
Delete a branch and its metadata from the stack. Children will be restacked onto the parent.

```bash
# Interactive selection of branch to delete
gs delete

# Delete specific branch
gs delete feat-old

# Delete without confirmation
gs delete -f feat-old

# Short aliases
gs d feat-old
gs rm feat-old
```

**What it does:**
- Deletes the specified branch (or current branch if none specified)
- Removes the branch from gs metadata
- Updates children to point to the deleted branch's parent
- Restacks all descendants
- Prompts for confirmation before proceeding

**Flags:**
- `-f, --force` - Delete without confirmation

**When to use:**
- Removing completed or abandoned feature branches
- Cleaning up your stack after merging to trunk
- Removing branches that are no longer needed
- Restructuring your stack by removing intermediate branches

**Aliases:** `d`, `remove`, `rm`

## Workflow Examples

### Creating a Stack of Features

```bash
# Start from trunk
git checkout main

# Create first feature branch
gs create feat-database
# ... make changes, commit ...

# Create second feature stacked on first
gs create feat-api
# ... make changes, commit ...

# Create third feature stacked on second
gs create feat-ui
# ... make changes, commit ...

# View your stack
gs log
```

### Navigating Your Stack

```bash
# View the stack
gs log

# Quickly jump to trunk
gs co -t

# Interactively select a branch to checkout
gs co

# Only see branches in current stack
gs co -s
```

### Tracking Existing Branches

```bash
# Checkout an existing branch
git checkout feat-existing

# Track it in gs
gs track
# Select parent branch when prompted

# Verify it's tracked
gs info
```

## Tips

1. **Use vim mode** for faster navigation if you're comfortable with j/k keys. Press ESC to toggle.
2. **Type to filter** in Select prompts to quickly find branches in large stacks.
3. **Use `gs co -t`** as a quick way to return to trunk from anywhere.
4. **Press Ctrl+C** anytime to safely cancel an operation.
5. **Check `gs log`** frequently to visualize your stack structure.
