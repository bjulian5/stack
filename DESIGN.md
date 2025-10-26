# Stack - Git-Native Stacked PR Tool

## Overview

**Stack** is a CLI tool for managing stacked pull requests on GitHub using a git-native approach. It allows developers to create, manage, and sync stacked PRs while using familiar git commands for most operations.

### Key Principles

1. **Git-native**: All operations use standard git primitives (branches, commits, rebase)
2. **Transparent**: Users can use regular git commands alongside the tool
3. **Bottom-up merging**: PRs must merge from bottom to top for predictability
4. **Multi-stack support**: Multiple stacks can coexist in one repository
5. **Gerrit-inspired**: Each commit represents one pull request
6. **Minimal metadata**: Only essential PR information stored in git trailers

### Architecture

- **Stack branch**: Single branch (e.g., `username/stack-auth-refactor/TOP`) containing all commits
- **Each commit = One PR**: Commit message becomes PR title/description
- **UUID branches**: Temporary branches (e.g., `username/stack-auth-refactor/550e8400`) for editing specific PRs
- **Git hooks**: Automatic metadata management via prepare-commit-msg and post-commit hooks
- **GitHub integration**: Uses `gh` CLI for PR operations

---

## Metadata Format

### Commit Message Structure

```
Add JWT authentication                    ← PR title (first line)
                                          ← blank line
Implements secure JWT-based auth with    ← PR description (body)
refresh tokens and cookie handling.

PR-UUID: 550e8400-e29b-41d4-a716
PR-Stack: auth-refactor
```

**Parsing rules:**
- First line → PR title
- Body (excluding trailers) → PR description
- `PR-UUID`: Unique identifier for this PR (16-char hex)
- `PR-Stack`: Name of the stack this PR belongs to

**No separate title/description fields needed** - the commit message IS the PR content.

---

## Core Workflows

### 1. Creating a Stack

**Command:**
```bash
git checkout main
git pull
stack new auth-refactor
```

**What happens:**
1. Creates branch `username/stack-auth-refactor/TOP` from current HEAD
2. Creates `.git/stack/auth-refactor/config.json` with stack metadata
3. Creates `.git/stack/auth-refactor/prs.json` for PR tracking
4. Installs git hooks (thin wrappers calling the binary)
5. Checks out the stack branch (current stack is determined by branch context)

**Output:**
```
✓ Created stack 'auth-refactor'
✓ Branch: username/stack-auth-refactor/TOP
✓ Base: main
✓ Installed git hooks
✓ Switched to stack branch
```

---

### 2. Adding PRs to Stack (Pure Git)

**User workflow:**
```bash
# Make changes
vim src/auth.go
git add src/auth.go

# Regular git commit - hook handles metadata
git commit
```

**Hook behavior** (`prepare-commit-msg`):
- Detects we're on a stack branch (matches `username/stack-*/TOP` pattern)
- Generates new UUID
- Opens editor with template:
  ```
  <cursor here - user types their commit message>

  PR-UUID: 550e8400-e29b-41d4-a716
  PR-Stack: auth-refactor
  ```

**User writes natural commit message:**
```
Add JWT authentication

Implements secure JWT-based auth with refresh tokens
and cookie handling. Uses RS256 algorithm for signing.
```

**Result:** Commit with full PR metadata ready to push.

---

### 3. Viewing Stacks

#### List All Stacks

**Command:**
```bash
stack list
```

**Output:**
```
Available stacks:

* auth-refactor      (3 PRs, base: main)
  feature-redesign   (5 PRs, base: main)
  bugfix-login       (1 PR, base: develop)

* = current stack
```

#### Show Current Stack

**Command:**
```bash
stack status
```

**Output:**
```
Stack: auth-refactor (username/stack-auth-refactor/TOP)
Base: origin/main

 #  Status    PR      Title                         Commit
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 1  🟢 Open   #1234   Add JWT authentication       abc1234
 2  🟡 Draft  #1235   Add refresh token handling   def5678
 3  ⚪ Local  -       Add cookie security          ghi9012

3 PRs total (1 open, 1 draft, 1 local)

Legend:
🟢 Open   - PR is open and ready for review
🟡 Draft  - PR is in draft state
🔵 Approved - PR has been approved
🟣 Merged - PR has been merged
⚪ Local  - Not yet pushed to GitHub
```

**Show specific stack:**
```bash
stack status feature-redesign
```

---

### 4. Switching Stacks (Fuzzy Finder)

**Command:**
```bash
stack switch
```

**Interactive fuzzy finder:**
```
> auth

  auth-refactor      (3 PRs, base: main)
  feature-redesign   (5 PRs, base: main)
  bugfix-login       (1 PR, base: develop)

3/3
>
```

Uses `github.com/ktr0731/go-fuzzyfinder` for interactive selection.

**Direct switch:**
```bash
stack switch feature-redesign
```

**What happens:**
1. Checks out the stack branch (current stack determined by branch)
2. Displays stack summary

---

### 5. Editing a PR in the Stack

#### Interactive Selection

**Command:**
```bash
stack edit
```

**Interactive fuzzy finder:**
Uses an interactive fuzzy finder to select which change to edit. Type to search by title, then press Enter to select.

```
> token

 #  Title                         Status
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 1  Add JWT authentication       🟢 #1234
 2  Add refresh token handling   🟡 #1235
 3  Add cookie security          ⚪ Local

3/3
>
```

#### What Happens

1. Tool extracts PR-UUID from selected commit
2. Creates branch `username/stack-auth-refactor/550e8400` at that commit
3. Checks out the UUID branch
4. User can now make changes

**Output:**
```
✓ Created branch username/stack-auth-refactor/550e8400
✓ Checked out PR #2: Add refresh token handling
✓ Make your changes and commit (amend to update, new commit to insert after)
```

---

### 6. UUID Branch Behavior

#### Case 1: Amend (Update Existing PR)

**Scenario:** User wants to update PR #2

```bash
# On username/stack-auth-refactor/550e8400
vim src/tokens.go
git add src/tokens.go
git commit --amend
```

**post-commit hook:**
1. Detects UUID branch (matches `username/stack-*/<uuid>` pattern)
2. Extracts stack name and UUID
3. Switches to stack branch
4. Finds commit with matching PR-UUID
5. Replaces that commit with amended version
6. Rebases subsequent commits (PR #3)
7. Switches back to UUID branch

**Output:**
```
✓ Updated PR #2 and rebased 1 subsequent PR
```

**Result:** Stack branch now has updated PR #2, and PR #3 is rebased on top.

---

#### Case 2: New Commit (Insert New PR After Current)

**Scenario:** User wants to add a new PR between #2 and #3

```bash
# On username/stack-auth-refactor/550e8400
vim src/middleware.go
git add src/middleware.go
git commit -m "Add auth middleware

Adds middleware to validate tokens on protected routes.
Integrates with JWT validation.
"
```

**post-commit hook:**
1. Detects UUID branch
2. Detects new commit (not an amend - no existing PR-UUID matches parent)
3. Generates new UUID for new commit
4. Adds PR-UUID and PR-Stack trailers to the commit
5. Switches to stack branch
6. Finds position of PR with UUID matching the branch
7. Inserts new commit AFTER that position
8. Rebases subsequent commits
9. Switches back to UUID branch

**Output:**
```
✓ Inserted new PR after #2, rebased 1 subsequent PR
```

**Result:** Stack now has 4 PRs:
```
1. Add JWT authentication
2. Add refresh token handling
3. Add auth middleware  ← new!
4. Add cookie security
```

---

### 7. Pushing to GitHub

**Command:**
```bash
stack push
```

**What happens:**

For each commit in the stack:
1. Extracts PR-UUID from commit
2. Creates branch `username/stack-<name>/<short-uuid>` (8 chars)
3. Cherry-picks all commits from base up to and including this one
4. Force-pushes branch to origin
5. Checks if PR exists (lookup UUID in `prs.json`)
   - **If exists:** Updates PR title/description via `gh pr edit`
   - **If new:** Creates PR via `gh pr create --draft`
6. Saves PR number and URL to `prs.json`
7. Sets PR base to previous PR's branch (or main for first PR)

**Output:**
```
Pushing stack 'auth-refactor'...

✓ 1/3 username/stack-auth-refactor/550e8400
      Updated PR #1234: Add JWT authentication
      https://github.com/user/repo/pull/1234

✓ 2/3 username/stack-auth-refactor/661f9511
      Created PR #1235: Add refresh token handling
      Base: username/stack-auth-refactor/550e8400
      https://github.com/user/repo/pull/1235

✓ 3/3 username/stack-auth-refactor/772fa622
      Created PR #1236: Add cookie security
      Base: username/stack-auth-refactor/661f9511
      https://github.com/user/repo/pull/1236

Done! View all PRs:
https://github.com/user/repo/pulls?q=is:pr+author:@me+head:username/stack-auth-refactor/
```

**Options:**
```bash
stack push --dry-run    # Show what would happen without doing it
stack push --force      # Force update stack visualizations even if unchanged
```

**Note:** To mark PRs as ready or draft, use `stack pr ready` or `stack pr draft` commands (see below).

**Implementation via `gh` CLI:**
```bash
# Create/update branch
git push origin username/stack-auth-refactor/550e8400 --force

# Create new PR
gh pr create \
  --title "Add JWT authentication" \
  --body "Implements secure JWT-based auth..." \
  --base main \
  --head username/stack-auth-refactor/550e8400 \
  --draft

# Update existing PR
gh pr edit 1234 \
  --title "Add JWT authentication (updated)" \
  --body "Implements secure JWT-based auth..."
```

---

### 8. After PRs Merge

**Command:**
```bash
stack refresh
```

**What happens:**
1. Fetches from origin
2. Queries PR state for each PR in stack via `gh pr view --json state,mergedAt`
3. Identifies merged PRs (must be from bottom of stack)
4. Removes merged commits from stack branch
5. Rebases remaining commits on updated base branch
6. Deletes merged PR branches
7. Updates `prs.json`

**Output:**
```
Checking for merged PRs...

✓ PR #1234 merged to main at 2025-10-19 16:30:00
  Removing commit abc1234

✓ PR #1235 merged to main at 2025-10-19 16:45:00
  Removing commit def5678

Rebasing remaining PRs on origin/main...
✓ Rebased 1 PR

Cleaning up branches...
✓ Deleted username/stack-auth-refactor/550e8400
✓ Deleted username/stack-auth-refactor/661f9511

Stack updated. 1 PR remaining.
```

**Error handling:**
- If a PR in the middle is merged (not bottom), tool errors and suggests user intervention
- Bottom-up merging is enforced for predictability

---

### 9. Stack Visualization in PRs

When you push PRs, Stack automatically adds a visualization comment to each PR showing the full stack context:

```markdown
## 📚 Stack: auth-refactor (3 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | #1234 | ✅ Open   | Add JWT authentication |
| 2 | #1235 | ✅ Open   | Add refresh token handling ← **YOU ARE HERE** |
| 3 | #1236 | 📝 Draft  | Add cookie security |

**Merge order:** `main → #1234 → #1235 → #1236`

---

💡 **Review tip:** Start from the bottom (#1234) for full context

<!-- stack-visualization: auth-refactor -->
```

**Features:**
- Shows full stack context in each PR
- Highlights current PR position
- Updates automatically on `stack push`
- Cached to avoid duplicate comments

---

### 10. Navigation Commands

Stack provides git-like navigation commands to move through your stack:

#### Move to Top
```bash
stack top
```
Moves to the TOP branch (all commits).

#### Move to Bottom
```bash
stack bottom
```
Moves to the first change (position 1).

#### Navigate Up/Down
```bash
# On UUID branch at position 2
stack up    # Move to position 3
stack down  # Move back to position 2

# From TOP branch
stack down  # Move to N-1 (second-to-last change)
```

**All navigation commands:**
- Validate uncommitted changes
- Sync with GitHub to show merge warnings
- Create/update UUID branches as needed

---

### 11. Fixup Workflow

Stack provides an interactive fixup command to quickly fix bugs in earlier changes:

```bash
# On TOP branch, make a fix
vim src/auth.go
git add src/auth.go

# Run fixup - opens fuzzy finder to select which change to fix
stack fixup
# Select: "2. Add refresh token handling"
# ✓ Creates fixup commit
# ✓ Runs autosquash rebase
# ✓ You remain on TOP branch
```

**Equivalent to:**
```bash
git commit --fixup <commit-hash>
git rebase -i --autosquash <parent-commit>
```

But with interactive fuzzy finder for change selection!

---

### 12. Rebase Recovery

If a rebase conflicts or gets aborted, Stack provides recovery tools:

**Scenario 1: Rebase conflicts**
```bash
stack restack
# Conflict! Fix conflicts...
git add resolved-file.txt
git rebase --continue

stack restack --recover
# ✓ Updated stack branch
# ✓ Updated UUID branches
# Rebase recovery complete!
```

**Scenario 2: Aborted rebase**
```bash
stack restack
# Conflicts are too complex, abort
git rebase --abort

stack restack --recover
# Options:
#   1. Retry rebase (recommended)
#   2. Restore to previous state (undo amend)
#   3. Keep current state (lose subsequent commits)
# Choose [1/2/3]: 1
# ✓ Successfully rebased
```

**Auto-retry:**
```bash
git rebase --abort
stack restack --recover --retry  # Skips prompts, retries immediately
```

---

### 13. Native Git Operations

Users can use regular git commands on the stack branch:

#### Rebase the Stack
```bash
git fetch origin
git rebase origin/main
```

#### Interactive Rebase (Reorder, Squash)
```bash
git rebase -i origin/main
# Reorder commits, squash multiple PRs together
# Hooks preserve PR metadata
```

#### Amend PR Metadata
```bash
git commit --amend
# Edit first line to change PR title
# Edit body to change PR description
# PR-UUID and PR-Stack preserved automatically

# Then push to update GitHub
stack push
```

#### View Stack Log
```bash
git log --oneline

abc1234 Add cookie security
def5678 Add refresh token handling
ghi9012 Add JWT authentication
```

---

## Commands Reference

### Installation

#### `stack install`
Install stack hooks and configure git.

```bash
stack install
```

**What it does:**
- Installs git hooks (prepare-commit-msg, post-commit, commit-msg)
- Configures git settings (core.commentChar=;)
- Idempotent operation (safe to run multiple times)

---

### Stack Management

#### `stack new <name>`
Create a new stack.

```bash
stack new auth-refactor
```

**Options:**
- `--base <branch>`: Set base branch (default: current branch)

---

#### `stack list`
List all stacks in the repository.

```bash
stack list
```

**Output:**
```
* auth-refactor      (3 PRs, base: main)
  feature-redesign   (5 PRs, base: main)
```

---

#### `stack status [name]`
Show status of current stack (or specified stack).

```bash
stack status
stack status feature-redesign
```

**Options:**
- `--verbose`: Show full PR descriptions

---

#### `stack switch [name]`
Switch to a different stack. If no name provided, opens fuzzy finder.

```bash
stack switch              # Interactive fuzzy finder
stack switch auth-refactor  # Direct switch
```

---

#### `stack delete [name]`
Delete a stack and its branches.

```bash
stack delete               # Delete current stack (with confirmation)
stack delete auth-refactor # Delete specific stack
stack delete --force       # Skip confirmation prompt
```

**What it does:**
- Deletes stack metadata from `.git/stack/<name>/`
- Removes stack branch (TOP branch)
- Removes all UUID branches for the stack
- Archives metadata to `.git/stack/.archived/<name>-<timestamp>/`

---

#### `stack cleanup`
Clean up stacks that have all PRs merged or are empty.

```bash
stack cleanup
```

**What it does:**
- Scans all stacks in repository
- Identifies stacks where all PRs are merged
- Identifies empty stacks with no commits
- Prompts for confirmation
- Deletes eligible stacks and their branches

---

### Working with Changes

#### `stack edit`
Edit a PR in the stack using an interactive fuzzy finder.

```bash
stack edit           # Opens interactive fuzzy finder
```

---

#### `stack top`
Move to the top of the stack (TOP branch).

```bash
stack top
```

---

#### `stack bottom`
Move to the bottom of the stack (first change).

```bash
stack bottom
```

---

#### `stack up`
Move up to the next change in the stack (higher position).

```bash
stack up    # From position 2 to position 3
```

---

#### `stack down`
Move down to the previous change in the stack (lower position).

```bash
stack down  # From TOP to N-1, or from position 3 to 2
```

---

#### `stack fixup`
Create a fixup commit for a selected change and autosquash.

```bash
git add .
stack fixup  # Interactive fuzzy finder to select change
```

**Requirements:**
- Must have staged changes
- Must be on TOP branch

---

### GitHub Integration

#### `stack push [options]`
Push PRs to GitHub.

```bash
stack push              # Push all PRs (creates as drafts by default)
stack push --dry-run    # Show what would happen without doing it
stack push --force      # Force update stack visualizations even if unchanged
```

---

#### `stack pr ready [--all]`
Mark changes as ready for review (not draft).

```bash
stack pr ready          # Mark current change as ready
stack pr ready --all    # Mark all changes in stack as ready
```

---

#### `stack pr draft [--all]`
Mark changes as draft.

```bash
stack pr draft          # Mark current change as draft
stack pr draft --all    # Mark all changes in stack as draft
```

---

#### `stack refresh`
Sync with GitHub to detect merged PRs and update stack.

```bash
stack refresh
```

**What it does:**
1. Fetches from remote
2. Queries GitHub for merge status
3. Validates bottom-up merging
4. Saves merged changes to metadata
5. Rebases remaining commits
6. Cleans up merged branches

---

#### `stack restack [options]`
Rebase the stack on top of the latest base branch.

```bash
stack restack                    # Fetch and rebase on current base
stack restack --onto develop     # Move stack to different base
stack restack --onto develop --fetch  # Fetch first, then move
stack restack --recover          # Recover from failed rebase
stack restack --recover --retry  # Retry failed rebase
```

**Options:**
- `--onto <branch>`: Rebase onto a different base branch
- `--fetch`: Fetch from remote before rebasing
- `--recover`: Recover from a failed or aborted rebase
- `--retry`: Retry the rebase (only with --recover)

---

#### `stack pr open [top]`
Open a PR in the browser.

```bash
stack pr open      # Interactive fuzzy finder
stack pr open top  # Open the top PR
```

---

### Hook Commands (Internal)

These are called by git hooks, not directly by users.

#### `stack hook prepare-commit-msg <file> <source> <sha>`

Called by git's `prepare-commit-msg` hook.

**Arguments (passed by Git):**
- `<file>`: Path to file containing commit message (e.g., `.git/COMMIT_EDITMSG`)
- `<source>`: Source of commit message (e.g., `message`, `template`, `merge`, `squash`)
- `<sha>`: Commit SHA when amending (optional)

**Example invocations:**
```bash
# New commit
stack hook prepare-commit-msg .git/COMMIT_EDITMSG message

# Amending existing commit
stack hook prepare-commit-msg .git/COMMIT_EDITMSG commit abc1234

# Template-based commit
stack hook prepare-commit-msg .git/COMMIT_EDITMSG template
```

#### `stack hook post-commit`

Called by git's `post-commit` hook. Takes no arguments.

**Example invocation:**
```bash
stack hook post-commit
```

#### `stack hook commit-msg <file>`

Called by git's `commit-msg` hook.

**Arguments (passed by Git):**
- `<file>`: Path to file containing commit message (e.g., `.git/COMMIT_EDITMSG`)

**Example invocation:**
```bash
stack hook commit-msg .git/COMMIT_EDITMSG
```

---

## Implementation Status

**Current Status:** Phases 1-5 Complete ✅

The tool has completed all core functionality:

- ✅ **Phase 1 (Foundation)** - Stack creation, listing, status display
- ✅ **Phase 2 (Git Hooks)** - Automatic UUID injection, amend/insert operations
- ✅ **Phase 3 (Editing & Navigation)** - Interactive editing, stack switching, navigation commands
- ✅ **Phase 4 (GitHub Integration)** - Push to GitHub, PR visualization, PR operations
- ✅ **Phase 5 (Sync & Refresh)** - Merge detection, stack rebasing, conflict recovery

See CLAUDE.md for detailed implementation information.

---

## Project Structure

```
stack/
├── main.go                      # Entry point, calls cmd.Execute()
├── go.mod
├── go.sum
├── README.md                    # Project overview, installation
├── DESIGN.md                    # This file (comprehensive user documentation)
├── CLAUDE.md                    # Development guidance for AI assistants
├── LICENSE
│
├── cmd/                         # CLI commands
│   ├── root.go                  # Root command setup with cobra
│   ├── command.go               # Command interface for registration pattern
│   ├── install/
│   │   └── install.go           # stack install command (✅ completed)
│   ├── newcmd/
│   │   └── new.go               # stack new command (✅ completed)
│   ├── list/
│   │   └── list.go              # stack list command (✅ completed)
│   ├── status/
│   │   └── status.go            # stack status command (✅ completed)
│   ├── edit/
│   │   └── edit.go              # stack edit command (✅ completed)
│   ├── fixup/
│   │   └── fixup.go             # stack fixup command (✅ completed)
│   ├── switch/
│   │   └── switch.go            # stack switch command (✅ completed)
│   ├── top/
│   │   └── top.go               # stack top command (✅ completed)
│   ├── bottom/
│   │   └── bottom.go            # stack bottom command (✅ completed)
│   ├── up/
│   │   └── up.go                # stack up command (✅ completed)
│   ├── down/
│   │   └── down.go              # stack down command (✅ completed)
│   ├── push/
│   │   └── push.go              # stack push command (✅ completed)
│   ├── refresh/
│   │   └── refresh.go           # stack refresh command (✅ completed)
│   ├── restack/
│   │   └── restack.go           # stack restack command (✅ completed)
│   ├── delete/
│   │   └── delete.go            # stack delete command (✅ completed)
│   ├── cleanup/
│   │   └── cleanup.go           # stack cleanup command (✅ completed)
│   ├── pr/
│   │   ├── pr.go                # Parent PR command (✅ completed)
│   │   ├── open/
│   │   │   └── open.go          # stack pr open command (✅ completed)
│   │   ├── ready/
│   │   │   └── ready.go         # stack pr ready command (✅ completed)
│   │   └── draft/
│   │       └── draft.go         # stack pr draft command (✅ completed)
│   └── hook/                    # Git hook implementations (✅ completed)
│       ├── hook.go              # Parent hook command
│       ├── prepare_commit_msg.go # prepare-commit-msg hook
│       ├── commit_msg.go        # commit-msg hook
│       ├── post_commit.go       # post-commit hook
│       └── operations.go        # Common hook operations
│
├── internal/                    # Internal packages
│   ├── git/                     # Git operations (✅ completed)
│   │   ├── client.go            # Client struct with git operations
│   │   ├── commit.go            # Commit and CommitMessage types with parsing
│   │   ├── rebase.go            # Rebase operations for stack updates
│   │   └── template.go          # Commit message templates
│   │
│   ├── model/                   # Domain models (✅ completed)
│   │   ├── stack.go             # Stack model
│   │   ├── change.go            # Change model
│   │   └── pr.go                # PR and PRData models with versioning
│   │
│   ├── stack/                   # Stack management (✅ completed)
│   │   ├── client.go            # Stack client for metadata management
│   │   ├── config.go            # Stack and global configuration
│   │   ├── context.go           # StackContext for branch-based state
│   │   ├── visualization.go     # Stack visualization in PR comments
│   │   └── rebase_state.go      # Rebase state management for recovery
│   │
│   ├── gh/                      # GitHub integration (✅ completed)
│   │   ├── client.go            # gh CLI wrapper with batch API
│   │   └── types.go             # GitHub types (PRSpec, Comment)
│   │
│   ├── ui/                      # User interface (✅ completed)
│   │   ├── config.go            # UI configuration settings
│   │   ├── format.go            # Formatting utilities
│   │   ├── styles.go            # lipgloss style definitions
│   │   ├── render.go            # Stack rendering functions
│   │   ├── status.go            # Status rendering
│   │   ├── select.go            # Interactive fuzzy finder
│   │   ├── table.go             # Table formatting
│   │   ├── tree.go              # Tree visualization
│   │   ├── input.go             # User input handling
│   │   ├── print.go             # Simple output functions
│   │   └── terminal.go          # Terminal utilities
│   │
│   ├── hooks/                   # Git hooks (✅ completed)
│   │   └── install.go           # Hook installation/uninstallation
│   │
│   └── common/                  # Common utilities
│       └── utils.go             # Shared utilities (username, UUID, etc.)
│
└── test/                        # Integration tests (future)
    ├── fixtures/                # Test git repos
    └── integration_test.go      # End-to-end tests
```

**Key architectural patterns implemented:**
- **Command interface**: Each command implements `Command` interface with `Register(parent *cobra.Command)` method
- **Dependency injection**: Commands receive `*git.Client`, `*stack.Client`, and `*gh.Client` instances at registration time
- **Package-per-command**: Each command lives in its own package for better organization
- **Git Client abstraction**: All git operations go through `git.Client` for consistency and testability
- **GitHub Client abstraction**: All GitHub operations use `gh.Client` wrapper around gh CLI

---

## Dependencies

**Required:**
- **gh CLI** - GitHub operations (install: https://cli.github.com/, authenticate: `gh auth login`)
- **Go 1.21+** - See `go.mod` for full dependency list

---

## Key Design Decisions

### 1. Why git trailers instead of separate files?

**Decision:** Store PR metadata (UUID, stack name) as git trailers in commit messages.

**Rationale:**
- Git-native: Metadata travels with commits during rebase, cherry-pick, etc.
- No sync issues: Metadata is part of the commit, can't get out of sync
- Simple: No need to parse separate files or maintain mappings
- Standard: Git trailers are a well-established convention

### 2. Why one commit = one PR?

**Decision:** Each commit on the stack branch represents exactly one PR.

**Rationale:**
- Gerrit-inspired: Proven workflow in large projects
- Clean history: Each PR is a logical unit of change
- Easy rebasing: Standard git rebase operations work naturally
- Familiar: Developers already think in terms of commits

### 3. Why UUID branches for editing?

**Decision:** Create temporary branches (e.g., `username/stack-<name>/<uuid>`) when editing a PR.

**Rationale:**
- Isolation: Changes don't affect the stack until committed
- Flexibility: Can make multiple commits, amend, etc.
- Automatic updates: Hooks handle updating the stack branch
- Git-native: Just normal branches, can use standard git commands

### 4. Why bottom-up merging only?

**Decision:** Enforce that PRs must merge from bottom to top.

**Rationale:**
- Predictability: Stack state is always well-defined
- Simplicity: No complex rebasing logic for out-of-order merges
- Safety: Reduces risk of conflicts and broken dependencies
- Common practice: Most stacked PR workflows enforce this

### 5. Why use gh CLI instead of GitHub API directly?

**Decision:** Use `gh` CLI as a subprocess instead of Go GitHub client library.

**Rationale:**
- Faster development: Don't need to handle auth, API details
- Authentication handled: User's existing `gh` auth works
- Simpler: No need to manage tokens, OAuth flows
- Feature parity: gh CLI has all features we need
- Trade-off: Slight performance cost, but acceptable for v1

### 6. Why hooks in the binary instead of bash scripts?

**Decision:** Implement hook logic in Go binary, call via thin wrappers.

**Rationale:**
- Single binary: Easier distribution and installation
- Shared code: Reuse parsing, git operations across hooks and commands
- Better error handling: Go's error handling > bash
- Testing: Can unit test hook logic
- Cross-platform: Works same on Windows, Mac, Linux

---

## Future Enhancements (Post v1.0)

1. **git push hook**: Trigger `stack push` automatically on `git push`
2. **PR templates**: Support custom PR templates
3. **Labels and reviewers**: Auto-assign labels and reviewers
4. **CI integration**: Wait for CI before marking ready
5. **Merge command**: `stack land` to merge via API with auto-refresh
6. **Split command**: `stack split <ref>` to split a commit into multiple PRs
7. **Sync command**: `stack sync` to pull remote changes into stack
8. **Dependencies**: Track dependencies between PRs across stacks
9. **Web UI**: Simple web interface to visualize stacks
10. **GitHub App**: Native GitHub integration for status checks

---

## FAQ

### Q: What happens if I manually edit the stack branch?

**A:** That's fine! The tool is designed to work with native git operations. Just make sure your commit messages have the required trailers (PR-UUID, PR-Stack). You can always run `stack push` to sync changes to GitHub.

### Q: Can I use this with other git forges (GitLab, Bitbucket)?

**A:** Currently GitHub-only via `gh` CLI. Future versions could support other forges by abstracting the GitHub operations.

### Q: What if I accidentally delete a UUID branch?

**A:** No problem! UUID branches are temporary. You can always run `stack edit` again to recreate it.

### Q: How do I resolve conflicts during rebase?

**A:** Just like normal git: fix conflicts, `git add` files, then `git rebase --continue`. The hooks will preserve your PR metadata.

### Q: Can I have multiple stacks from different base branches?

**A:** Yes! Each stack can have its own base branch (main, develop, etc.). Set with `stack new <name> --base <branch>`.

### Q: What if someone else merges a PR out of order?

**A:** The tool will error and suggest manual intervention. Bottom-up merging is enforced for safety.
