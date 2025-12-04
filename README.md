# Stack

A git-native CLI tool for managing stacked pull requests on GitHub. Stack treats **each commit as a pull request**, making it easy to break complex features into small, focused changes.

## Table of Contents

- [Why Stack?](#why-stack)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Basic Usage](#basic-usage)
- [Common Workflows](#common-workflows)
- [Command Reference](#command-reference)
- [Troubleshooting](#troubleshooting)
- [Tips & Best Practices](#tips--best-practices)

---

## Why Stack?

**The Problem:** Complex features require multiple dependent PRs, but managing them manually is tedious. Large PRs are hard to review, and when PR #1 changes, you manually rebase PR #2, PR #3, etc.

**The Solution:** Stack manages stacked PRs using standard git. Each commit on your branch becomes one PR on GitHub, with dependencies automatically managed.

```
Your branch (5 commits):
    main → [Commit 1] → [Commit 2] → [Commit 3] → [Commit 4] → [Commit 5]
            ↓            ↓            ↓            ↓            ↓
          PR #1        PR #2        PR #3        PR #4        PR #5
```

**Key Benefits:**
- **Git-native**: Use regular `git commit` and `git rebase` - Stack adds automation
- **Smart rebasing**: Edit any commit - Stack rebases everything else automatically
- **Visual context**: Each PR shows the full stack structure
- **Conflict recovery**: Built-in recovery for failed rebases

---

## Installation

### Prerequisites

Ensure you have:
- [Git](https://git-scm.com/downloads) (2.0+)
- [GitHub CLI](https://cli.github.com/) (`gh`)
- GitHub authentication: `gh auth login`

### Install

```bash
# Install Stack
go install github.com/bjulian5/stack@latest

# In your repository, install git hooks
cd /path/to/your/repo
stack install
```

Enable shell completion (optional):
```bash
source <(stack completion bash)  # or zsh, fish
```

---

## Quick Start

```bash
# 1. Create a stack
git checkout main
stack new my-feature

# 2. Add commits (each becomes a PR)
git commit -m "Add database migration"
git commit -m "Add API endpoint"
git commit -m "Add frontend component"

# 3. Push to GitHub (creates 3 linked PRs)
stack push

# 4. View your stack
stack status

# 5. Edit a PR (uses fuzzy finder)
stack edit
git commit --amend
stack top

# 6. Push updates
stack push

# 7. After PRs merge, sync your stack
stack refresh
```

---

## Core Concepts

### One Commit = One PR

Each commit represents exactly one pull request:
- **Amending commits** = updating PRs
- **Reordering commits** = reordering PRs
- **Cherry-picking commits** = backporting PRs

### Git Trailers

Stack stores metadata as git trailers in commit messages (automatically added by git hooks):

```
Add JWT authentication

Implements secure JWT-based authentication.

PR-UUID: 550e8400-e29b-41d4-a716
PR-Stack: auth-refactor
```

Trailers travel with commits through rebase and amend operations, creating an immutable link between commits and PRs.

### Branch Types

**TOP Branch** (your working branch)
- Format: `username/stack-<name>/TOP`
- Example: `bjulian5/stack-auth-refactor/TOP`
- Contains all commits in the stack

**UUID Branch** (temporary editing branch)
- Format: `username/stack-<name>/<uuid>`
- Created automatically when editing specific commits
- Cleaned up automatically

### Bottom-Up Merging

PRs must merge in order from bottom to top. Merging out of order breaks dependencies, so Stack validates this when you run `stack refresh`.

---

## Basic Usage

### Creating a Stack

```bash
stack new my-feature              # Use current branch as base
stack new my-feature --base main  # Specify base branch
```

### Adding Changes

Use regular git:
```bash
git commit -m "Your change"
```

Git hooks automatically add metadata to each commit.

### Viewing Your Stack

```bash
stack status                      # Current stack
stack status my-feature           # Specific stack
stack status --verbose            # Show full descriptions
```

Example output:
```
auth-refactor
╰─ main
   ├─ ◆ #1234 Add JWT authentication (abc1234)
   ├─ ◆ #1235 Add refresh token rotation (def5678)
   ╰─ ● #1236 Add cookie security (ghi9012) [needs push] ←
```

Legend: `◆` = pushed to GitHub, `●` = needs push, `←` = current position

### Navigating Your Stack

```bash
stack top        # Move to top of stack
stack bottom     # Move to first commit
stack up         # Move up one change
stack down       # Move down one change
stack edit       # Interactive fuzzy finder
```

### Pushing to GitHub

```bash
stack push              # Push changed PRs
stack push --dry-run    # Preview changes
stack push --force      # Force push all PRs
```

Stack creates new PRs as drafts by default.

---

## Common Workflows

### Editing a Change

```bash
# Navigate to the change
stack edit  # Use fuzzy finder

# Make your changes
vim src/auth.go
git add src/auth.go

# Amend the commit (Stack auto-rebases subsequent commits)
git commit --amend

# Return to top and push
stack top
stack push
```

### Inserting a New Change

```bash
# Navigate to insertion point
stack edit  # Select commit #2

# Create NEW commit (not amend)
git commit -m "Add password hashing"

# Stack inserts it after current position and rebases remaining commits

# Return to top and push
stack top
stack push
```

### Quick Bug Fixes

```bash
# Fix the bug
vim src/auth.go
git add src/auth.go

# Use fixup to squash into earlier commit
stack fixup  # Select target commit from fuzzy finder

# Push updates
stack push
```

### Handling Merged PRs

```bash
stack refresh
```

Stack automatically:
- Detects merged PRs from GitHub
- Removes merged commits from your stack
- Rebases remaining commits on updated base branch
- Updates base branches for remaining PRs

### Rebasing on Base Branch

```bash
stack restack                # Rebase on latest base
stack restack --fetch        # Fetch first, then rebase
stack restack --onto develop # Move to different base

# If conflicts occur and you abort:
stack restack --recover      # Choose retry or restore
```

### Managing PR Status

```bash
stack pr ready       # Mark current change as ready
stack pr draft       # Mark current change as draft
stack pr ready --all # Mark all changes as ready
stack pr draft --all # Mark all changes as draft
```

### Opening PRs

```bash
stack pr open              # Open current PR
stack pr open top          # Open top PR
stack pr open --select     # Fuzzy finder
```

### Managing Stacks

```bash
stack list                 # List all stacks
stack switch              # Interactive switcher
stack switch my-feature   # Direct switch
stack delete my-feature   # Delete stack
stack cleanup             # Clean up merged stacks
```

---

## Command Reference

### Stack Management
- `stack new <name> [--base <branch>]` - Create a new stack
- `stack list` - List all stacks
- `stack status [name] [--verbose]` - Show stack status
- `stack switch [name]` - Switch between stacks
- `stack delete [name] [--force]` - Delete a stack
- `stack cleanup` - Clean up fully merged stacks

### Navigation
- `stack top` - Move to top of stack
- `stack bottom` - Move to first commit
- `stack up` - Move up one change
- `stack down` - Move down one change
- `stack edit` - Interactive picker

### Editing
- `git commit` - Add a new change
- `git commit --amend` - Update current change
- `stack fixup` - Create fixup commit

### GitHub Integration
- `stack push [--dry-run] [--force]` - Push stack to GitHub
- `stack refresh` - Sync with GitHub and detect merged PRs
- `stack restack [--fetch] [--onto <branch>] [--recover]` - Rebase on base branch

### PR Management
- `stack pr ready [--all]` - Mark changes as ready for review
- `stack pr draft [--all]` - Mark changes as draft
- `stack pr open [top] [--select]` - Open PRs in browser

### Setup
- `stack install` - Install hooks and configure git
- `stack completion <shell>` - Generate shell completion
- `stack help [command]` - Show help

---

## Troubleshooting

### "Uncommitted changes detected"

Commit or stash your changes before running Stack commands:
```bash
git stash
stack <command>
git stash pop
```

### "Out-of-order merge detected"

A PR was merged out of order. Either:
1. Merge earlier PRs first (recommended)
2. Revert the out-of-order merge on GitHub

### Rebase conflicts

```bash
# Option 1: Resolve manually
vim <file>
git add <file>
git rebase --continue

# Option 2: Abort and recover
git rebase --abort
stack restack --recover
```

### Git hooks not running

```bash
stack install
chmod +x .git/hooks/prepare-commit-msg
chmod +x .git/hooks/post-commit
chmod +x .git/hooks/commit-msg
```

### PRs not updating

```bash
stack push --force       # Force update all PRs
gh auth status           # Verify authentication
```

For more help, see [GitHub Issues](https://github.com/bjulian5/stack/issues).

---

## Tips & Best Practices

### Recommended Git Aliases

Add to `~/.gitconfig`:
```gitconfig
[alias]
    # e.g. git fixup <rev> will update the target commit
	fixup = !sh -c 'REV=$(git rev-parse $1) && git commit --fixup $@ && GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash $REV^' -
```




### Recovery Mechanisms

Stack includes safety features:
- **Rebase state recovery**: `stack restack --recover`
- **Archived stacks**: Deleted stacks saved in `.git/stack/.archived/`
- **Git reflog**: All commits recoverable via `git reflog`

---

## How It Works

Stack uses git hooks to automate metadata management. When you commit, hooks automatically add unique IDs to your commits. When you amend or insert commits, hooks rebase subsequent commits automatically.

Stack stores metadata in `.git/stack/<stack-name>/` and uses the `gh` CLI for all GitHub operations.

---

## Further Reading

- [GitHub Issues](https://github.com/bjulian5/stack/issues) - Report bugs or request features
- [Gerrit Code Review](https://www.gerritcodereview.com/) - Inspiration for the one-commit-per-PR model

