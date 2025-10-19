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

- **Stack branch**: Single branch (e.g., `username/stack-auth-refactor`) containing all commits
- **Each commit = One PR**: Commit message becomes PR title/description
- **UUID branches**: Temporary branches (e.g., `username/stack-auth-refactor-550e8400`) for editing specific PRs
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
1. Creates branch `username/stack-auth-refactor` from current HEAD
2. Creates `.git/stack/auth-refactor/config.json` with stack metadata
3. Creates `.git/stack/auth-refactor/prs.json` for PR tracking
4. Installs git hooks (thin wrappers calling the binary)
5. Sets as current stack (`.git/stack/current` → `auth-refactor`)
6. Checks out the stack branch

**Output:**
```
✓ Created stack 'auth-refactor'
✓ Branch: username/stack-auth-refactor
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
- Detects we're on a stack branch (matches `username/stack-*` pattern)
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
stack show
```

**Output:**
```
Stack: auth-refactor (username/stack-auth-refactor)
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
stack show feature-redesign
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
1. Updates `.git/stack/current` to new stack name
2. Checks out the stack branch
3. Displays stack summary

---

### 5. Editing a PR in the Stack

#### Interactive Selection

**Command:**
```bash
stack edit
```

**Interactive prompt:**
```
Select PR to edit:

 #  Title                         Status
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 1  Add JWT authentication       🟢 #1234
 2  Add refresh token handling   🟡 #1235
 3  Add cookie security          ⚪ Local

Enter number: 2
```

**Direct selection:**
```bash
stack edit 2           # Edit 2nd PR
stack edit abc1234     # Edit by commit hash
```

#### What Happens

1. Tool extracts PR-UUID from selected commit
2. Creates branch `username/stack-auth-refactor-550e8400` at that commit
3. Checks out the UUID branch
4. User can now make changes

**Output:**
```
✓ Created branch username/stack-auth-refactor-550e8400
✓ Checked out PR #2: Add refresh token handling
✓ Make your changes and commit (amend to update, new commit to insert after)
```

---

### 6. UUID Branch Behavior

#### Case 1: Amend (Update Existing PR)

**Scenario:** User wants to update PR #2

```bash
# On username/stack-auth-refactor-550e8400
vim src/tokens.go
git add src/tokens.go
git commit --amend
```

**post-commit hook:**
1. Detects UUID branch (matches `username/stack-*-<uuid>` pattern)
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
# On username/stack-auth-refactor-550e8400
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
2. Creates branch `username/stack-<name>-<short-uuid>` (8 chars)
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

✓ 1/3 username/stack-auth-refactor-550e8400
      Updated PR #1234: Add JWT authentication
      https://github.com/user/repo/pull/1234

✓ 2/3 username/stack-auth-refactor-661f9511
      Created PR #1235: Add refresh token handling
      Base: username/stack-auth-refactor-550e8400
      https://github.com/user/repo/pull/1235

✓ 3/3 username/stack-auth-refactor-772fa622
      Created PR #1236: Add cookie security
      Base: username/stack-auth-refactor-661f9511
      https://github.com/user/repo/pull/1236

Done! View all PRs:
https://github.com/user/repo/pulls?q=is:pr+author:@me+head:username/stack-auth-refactor-
```

**Options:**
```bash
stack push --ready      # Mark all PRs as ready for review (not draft)
stack push --pr 2       # Push only PR #2 (and update bases for #3+)
stack push --dry-run    # Show what would happen without doing it
```

**Implementation via `gh` CLI:**
```bash
# Create/update branch
git push origin username/stack-auth-refactor-550e8400 --force

# Create new PR
gh pr create \
  --title "Add JWT authentication" \
  --body "Implements secure JWT-based auth..." \
  --base main \
  --head username/stack-auth-refactor-550e8400 \
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
✓ Deleted username/stack-auth-refactor-550e8400
✓ Deleted username/stack-auth-refactor-661f9511

Stack updated. 1 PR remaining.
```

**Error handling:**
- If a PR in the middle is merged (not bottom), tool errors and suggests user intervention
- Bottom-up merging is enforced for predictability

---

### 9. Native Git Operations

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

#### Use Git Fixup
```bash
# On stack branch
git log --oneline
# abc1234 Add cookie security
# def5678 Add refresh tokens
# ghi9012 Add JWT auth

# Fix bug in JWT auth commit
vim src/auth.go
git add src/auth.go
git fixup ghi9012

# Creates fixup commit and auto-squashes
# Hook preserves PR-UUID metadata
```

#### Amend PR Metadata
```bash
git commit --amend
# Edit first line to change PR title
# Edit body to change PR description
# PR-UUID and PR-Stack preserved automatically

# Then push to update GitHub
stack push --pr 1
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

#### `stack show [name]`
Show details of current stack (or specified stack).

```bash
stack show
stack show feature-redesign
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

#### `stack delete <name>`
Delete a stack and its branches.

```bash
stack delete auth-refactor
```

**Flags:**
- `--force`: Delete even if PRs are open
- `--keep-branches`: Delete stack metadata but keep branches

---

### Working with PRs

#### `stack edit [ref]`
Edit a PR in the stack. Opens interactive selector if no ref provided.

```bash
stack edit           # Interactive
stack edit 2         # Edit PR #2
stack edit abc1234   # Edit by commit hash
```

---

#### `stack push [options]`
Push PRs to GitHub.

```bash
stack push                # Push all PRs
stack push --pr 2        # Push only PR #2
stack push --ready       # Mark all as ready for review
stack push --dry-run     # Show what would happen
```

---

#### `stack refresh`
Sync with GitHub to detect merged PRs and update stack.

```bash
stack refresh
```

---

### Utilities

#### `stack status`
Show current state of the stack.

```bash
stack status
```

**Output:**
```
Current stack: auth-refactor
Branch: username/stack-auth-refactor
Base: main (up to date)
Uncommitted changes: none
PRs: 3 total (1 open, 1 draft, 1 local)
Needs sync: no
```

---

#### `stack open [ref]`
Open PR in browser.

```bash
stack open      # Opens first local PR or prompts
stack open 2    # Opens PR #2
```

---

#### `stack config <key> [value]`
Get or set configuration.

```bash
stack config username              # Get username
stack config username johndoe      # Set username
stack config --list                # List all config
```

**Supported configs:**
- `username`: GitHub username (for branch names)
- `base`: Default base branch
- `hooks.enabled`: Enable/disable hooks
- `hooks.auto-rebase`: Auto-rebase after UUID branch commits

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

## Git Hooks Implementation

### Hook Installation

When running `stack new`, the tool installs three git hooks as thin bash wrappers that delegate to the `stack` binary:

**`.git/hooks/prepare-commit-msg`:**
```bash
#!/bin/bash
# Git calls this with: prepare-commit-msg <COMMIT_MSG_FILE> <source> [<sha>]
# We pass all arguments to the stack binary
exec stack hook prepare-commit-msg "$@"
```

**`.git/hooks/post-commit`:**
```bash
#!/bin/bash
# Git calls this with no arguments after a commit is created
exec stack hook post-commit "$@"
```

**`.git/hooks/commit-msg`:**
```bash
#!/bin/bash
# Git calls this with: commit-msg <COMMIT_MSG_FILE>
# We pass all arguments to the stack binary
exec stack hook commit-msg "$@"
```

**Why thin wrappers?**
- All hook logic lives in the Go binary (easier to test and maintain)
- Single binary distribution (no separate hook scripts to manage)
- Cross-platform compatibility

---

### Hook: prepare-commit-msg

**Triggers:** Before commit message editor opens

**Behavior:**
1. Check if on a stack branch (matches `username/stack-*`)
2. If not, exit (do nothing)
3. Generate new UUID
4. Check if this is an amend (commit already has PR-UUID)
   - If amend: preserve existing UUID
   - If new: use generated UUID
5. Append trailers to commit message template

**Algorithm:**
```
1. Get current branch
2. If not stack branch, exit
3. Read commit message file
4. If already has PR-UUID trailer, exit (amend case)
5. Generate UUID
6. Extract stack name from branch
7. Append trailers:
   - PR-UUID: <uuid>
   - PR-Stack: <stack-name>
8. Write back to file
```

---

### Hook: post-commit

**Triggers:** After commit is created

**Behavior on UUID branch:**
1. Detect UUID branch (matches `username/stack-<name>-<uuid>`)
2. Get the commit that was just made
3. Determine if this is an amend or new commit:
   - **Amend:** Commit has PR-UUID matching the branch UUID
   - **New commit:** Commit doesn't have PR-UUID, or UUID doesn't match
4. Switch to stack branch
5. **If amend:**
   - Find commit with matching UUID in stack
   - Replace that commit (via rebase --onto)
   - Rebase subsequent commits
6. **If new commit:**
   - Generate UUID and add trailers to commit
   - Find position of commit with branch UUID
   - Insert new commit after that position
   - Rebase subsequent commits
7. Switch back to UUID branch

**Algorithm:**
```
1. Get current branch
2. If not UUID branch, exit
3. Parse branch: extract stack name and UUID
4. Get HEAD commit
5. Check if HEAD has PR-UUID matching branch UUID
6. Switch to stack branch (username/stack-<name>)

IF AMEND:
  7a. Find commit position with matching UUID
  8a. Use git rebase --onto to replace that commit
  9a. Rebase commits after position

IF NEW:
  7b. Add PR-UUID and PR-Stack trailers to commit
  8b. Find commit position with branch UUID
  9b. Insert new commit at position + 1
  10b. Rebase remaining commits

11. Switch back to UUID branch
12. Print status message
```

---

### Hook: commit-msg

**Triggers:** After commit message is written, before commit is created

**Behavior:**
1. Check if on stack or UUID branch
2. If not, exit
3. Validate commit message:
   - Has PR-UUID trailer
   - Has PR-Stack trailer
   - First line (PR title) is not empty
4. If validation fails, abort commit with error

**Algorithm:**
```
1. Get current branch
2. If not stack/UUID branch, exit
3. Read commit message
4. Parse git trailers
5. Validate:
   - PR-UUID exists
   - PR-Stack exists
   - First line not empty
6. If invalid, exit with error code
```

---

## Data Structures

### Stack Config

**Location:** `.git/stack/<stack-name>/config.json`

**Schema:**
```json
{
  "name": "auth-refactor",
  "branch": "username/stack-auth-refactor",
  "base": "main",
  "created": "2025-10-19T15:00:00Z",
  "last_synced": "2025-10-19T16:00:00Z"
}
```

**Fields:**
- `name`: Stack name (user-provided)
- `branch`: Full branch name
- `base`: Base branch for PRs (e.g., "main")
- `created`: ISO 8601 timestamp
- `last_synced`: Last time `stack push` was run

---

### PR Tracking

**Location:** `.git/stack/<stack-name>/prs.json`

**Schema:**
```json
{
  "550e8400-e29b-41d4-a716": {
    "pr_number": 1234,
    "url": "https://github.com/user/repo/pull/1234",
    "branch": "username/stack-auth-refactor-550e8400",
    "created_at": "2025-10-19T15:00:00Z",
    "last_pushed": "2025-10-19T16:00:00Z",
    "state": "open"
  },
  "661f9511-e29b-41d4-a716": {
    "pr_number": 1235,
    "url": "https://github.com/user/repo/pull/1235",
    "branch": "username/stack-auth-refactor-661f9511",
    "created_at": "2025-10-19T15:05:00Z",
    "last_pushed": "2025-10-19T16:00:00Z",
    "state": "draft"
  }
}
```

**Fields:**
- Key: Full UUID (16 hex chars)
- `pr_number`: GitHub PR number
- `url`: Full PR URL
- `branch`: Branch name for this PR
- `created_at`: When PR was first created
- `last_pushed`: Last time this PR was pushed
- `state`: PR state ("open", "draft", "closed", "merged")

---

### Current Stack

**Location:** `.git/stack/current`

**Content:** Single line with current stack name
```
auth-refactor
```

---

## Implementation Plan

### Phase 1: Foundation (Week 1)

**Goal:** Basic CLI and git operations working

**Tasks:**
1. Go project setup
   - Module: `github.com/username/stack`
   - CLI framework: `cobra`
   - Dependencies:
     - `github.com/go-git/go-git/v5` (git operations)
     - `github.com/ktr0731/go-fuzzyfinder` (fuzzy finder)
     - `github.com/charmbracelet/lipgloss` (terminal styling)
     - `github.com/google/uuid` (UUID generation)

2. Core git operations (`internal/git/`)
   - `getCurrentBranch()` - get current branch name
   - `getCommits(branch)` - get all commits on branch
   - `parseCommitMessage(msg)` → title, body, trailers
   - `extractTrailer(msg, key)` - get trailer value
   - `createBranch(name, commit)` - create branch at commit
   - `cherryPickCommits(commits)` - cherry-pick range
   - `rebaseCommits(from, onto)` - rebase operation

3. Stack metadata (`internal/stack/`)
   - `Stack` struct
   - `PR` struct
   - `loadStack(name)` - load stack config from disk
   - `saveStack(stack)` - save stack config
   - `loadPRs(stackName)` - load PR tracking
   - `savePRs(stackName, prs)` - save PR tracking
   - `getCurrentStack()` - read `.git/stack/current`
   - `setCurrentStack(name)` - write `.git/stack/current`

4. Basic commands (`cmd/`)
   - `stack new <name>` - create new stack
   - `stack list` - list all stacks
   - `stack show [name]` - show stack details

**Deliverable:** Can create stacks and view them

---

### Phase 2: Hooks (Week 1-2)

**Goal:** Git hooks for automatic metadata management

**Tasks:**
1. Hook installation (`internal/hooks/`)
   - `installHooks()` - create wrapper scripts in `.git/hooks/`
   - `uninstallHooks()` - remove hooks
   - `checkHooksInstalled()` - verify hooks are present

2. Hook implementations
   - `prepare-commit-msg` (`cmd/hook.go`)
     - Detect stack branch
     - Generate UUID
     - Add trailers to commit message
     - Handle amend case (preserve UUID)

   - `post-commit` (`cmd/hook.go`)
     - Detect UUID branch
     - Determine amend vs new commit
     - Update stack branch accordingly
     - Rebase subsequent commits
     - Handle errors gracefully

   - `commit-msg` (`cmd/hook.go`)
     - Validate PR metadata presence
     - Validate format (non-empty title)
     - Exit with error if invalid

3. Git operations for hooks
   - `replaceCommit(uuid, newCommit)` - replace commit in stack
   - `insertCommitAfter(uuid, newCommit)` - insert new PR
   - `rebaseFrom(position)` - rebase from position to end

4. Testing
   - Create test git repo
   - Test prepare-commit-msg on stack branch
   - Test post-commit amend flow
   - Test post-commit new commit (insertion) flow
   - Test commit-msg validation

**Deliverable:** Hooks work, can add PRs with `git commit`, can edit and insert via UUID branches

---

### Phase 3: Editing & Navigation (Week 2)

**Goal:** Interactive PR editing and stack switching

**Tasks:**
1. `stack edit` command (`cmd/edit.go`)
   - Interactive PR selection (numbered list)
   - Extract UUID from selected commit
   - Create UUID branch at that commit
   - Checkout UUID branch
   - Support direct ref: `stack edit 2`, `stack edit abc1234`

2. `stack switch` command (`cmd/switch.go`)
   - Integrate fuzzy finder (`go-fuzzyfinder`)
   - List all stacks with metadata
   - Allow filtering by name
   - Update current stack
   - Checkout stack branch
   - Support direct switch: `stack switch <name>`

3. Better output (`internal/ui/`)
   - Colored status indicators (🟢🟡🔵🟣⚪)
   - Table formatting for `stack show`
   - Box drawing for visual separation
   - Progress indicators
   - Terminal width detection

**Deliverable:** Can easily navigate between stacks and edit PRs

---

### Phase 4: GitHub Integration (Week 2-3)

**Goal:** Push to GitHub and sync state

**Tasks:**
1. GitHub username detection (`internal/config/`)
   - Parse from git config (`github.user`)
   - Parse from `gh` CLI config
   - Allow manual config: `stack config username <name>`
   - Store in `.git/stack/config.json`

2. `gh` CLI integration (`internal/github/`)
   - `execGH(args)` - wrapper to execute gh commands
   - `createPR(title, body, base, head, draft)` - create PR
   - `updatePR(number, title, body)` - update PR
   - `getPRState(number)` - get PR state
   - Parse JSON output from gh CLI

3. `stack push` command (`cmd/push.go`)
   - Iterate commits in stack
   - For each commit:
     - Parse commit message → title, body, UUID
     - Create branch `username/stack-<name>-<short-uuid>`
     - Cherry-pick commits from base up to this one
     - Push branch to origin (force)
     - Check if PR exists (lookup in prs.json)
     - Create or update PR via `gh`
     - Set PR base to previous PR's branch
     - Save PR number in prs.json
   - Handle `--pr <n>` flag (push single PR)
   - Handle `--ready` flag (mark as ready for review)
   - Handle `--dry-run` flag

4. PR state tracking
   - Update `prs.json` after push
   - Store PR numbers, URLs, state
   - Track last push time

5. Error handling
   - Handle gh CLI not installed
   - Handle not authenticated
   - Handle API rate limits
   - Handle network errors

**Deliverable:** Can push stacks to GitHub and create/update PRs

---

### Phase 5: Sync & Refresh (Week 3)

**Goal:** Handle merged PRs and keep stack updated

**Tasks:**
1. `stack refresh` command (`cmd/refresh.go`)
   - Fetch from origin
   - For each PR in prs.json:
     - Query state via `gh pr view <number> --json state,mergedAt`
     - Detect merged PRs
   - Validate merged PRs are from bottom of stack
   - Remove merged commits from stack branch
   - Rebase remaining commits on latest base
   - Delete merged PR branches (local and remote)
   - Update prs.json (remove merged PRs)
   - Display summary

2. `stack rebase` command (`cmd/rebase.go`)
   - Wrapper around `git rebase`
   - Fetch latest base branch
   - Rebase stack branch on base
   - Validate hooks preserved metadata
   - Update PR bases if needed

3. Merge detection
   - Parse merged PR data from GitHub
   - Match merged commits to stack commits
   - Handle out-of-order merges (error)

**Deliverable:** Can detect merged PRs and clean up stack automatically

---

### Phase 6: Polish & UX (Week 4)

**Goal:** Production-ready tool with great UX

**Tasks:**
1. Error handling
   - Graceful failures with helpful messages
   - Recovery suggestions ("Try: stack refresh")
   - Validation before destructive operations
   - Detect dirty working directory

2. Validation & safety
   - Check for uncommitted changes before operations
   - Warn before force-push
   - Confirm before delete
   - Detect conflicts during rebase
   - Provide recovery instructions

3. Configuration (`cmd/config.go`)
   - `stack config list` - show all config
   - `stack config get <key>` - get value
   - `stack config set <key> <value>` - set value
   - Supported configs:
     - `username` - GitHub username
     - `base` - Default base branch
     - `hooks.enabled` - Enable/disable hooks
     - `hooks.auto-rebase` - Auto-rebase on UUID branch commits

4. Additional commands
   - `stack status` - show current state
   - `stack open [ref]` - open PR in browser
   - `stack delete <name>` - delete stack

5. Help and documentation
   - Comprehensive help text for all commands
   - Examples in help output
   - README with quickstart
   - User guide (USAGE.md)

6. Testing
   - Unit tests for core logic
   - Integration tests with real git repo
   - Test error conditions
   - Test hook edge cases

7. Performance
   - Optimize git operations
   - Cache PR state locally
   - Minimize GitHub API calls

**Deliverable:** Polished, production-ready CLI tool

---

## Project Structure

```
stack/
├── main.go                      # Entry point
├── go.mod
├── go.sum
├── README.md                    # Project overview, installation
├── DESIGN.md                    # This file
├── USAGE.md                     # User guide (created in Phase 6)
├── LICENSE
│
├── cmd/                         # CLI commands
│   ├── root.go                  # Root command setup
│   ├── new.go                   # stack new
│   ├── list.go                  # stack list
│   ├── show.go                  # stack show
│   ├── switch.go                # stack switch
│   ├── edit.go                  # stack edit
│   ├── push.go                  # stack push
│   ├── refresh.go               # stack refresh
│   ├── rebase.go                # stack rebase
│   ├── status.go                # stack status
│   ├── open.go                  # stack open
│   ├── delete.go                # stack delete
│   ├── config.go                # stack config
│   └── hook.go                  # stack hook (subcommands)
│
├── internal/                    # Internal packages
│   ├── git/                     # Git operations
│   │   ├── operations.go        # Core git operations
│   │   ├── commit.go            # Commit parsing and manipulation
│   │   ├── branch.go            # Branch operations
│   │   └── rebase.go            # Rebase operations
│   │
│   ├── stack/                   # Stack management
│   │   ├── stack.go             # Stack struct and operations
│   │   ├── pr.go                # PR struct and tracking
│   │   └── metadata.go          # Load/save metadata (config.json, prs.json)
│   │
│   ├── github/                  # GitHub integration
│   │   ├── client.go            # gh CLI wrapper
│   │   ├── pr.go                # PR operations (create, update, query)
│   │   └── auth.go              # Authentication helpers
│   │
│   ├── hooks/                   # Git hooks
│   │   ├── install.go           # Hook installation/uninstallation
│   │   ├── prepare_commit_msg.go  # prepare-commit-msg logic
│   │   ├── post_commit.go       # post-commit logic
│   │   └── commit_msg.go        # commit-msg validation
│   │
│   ├── ui/                      # User interface
│   │   ├── table.go             # Table rendering
│   │   ├── prompt.go            # User prompts and input
│   │   ├── fuzzy.go             # Fuzzy finder integration
│   │   ├── progress.go          # Progress indicators
│   │   └── format.go            # Text formatting and colors
│   │
│   └── config/                  # Configuration
│       ├── config.go            # Global config management
│       └── defaults.go          # Default values
│
└── test/                        # Integration tests
    ├── fixtures/                # Test git repos
    └── integration_test.go      # End-to-end tests
```

---

## Dependencies

### go.mod

```go
module github.com/username/stack

go 1.21

require (
    github.com/spf13/cobra v1.8.0              // CLI framework
    github.com/go-git/go-git/v5 v5.11.0        // Git operations
    github.com/ktr0731/go-fuzzyfinder v0.7.0   // Fuzzy finder for stack switching
    github.com/charmbracelet/lipgloss v0.9.1   // Terminal styling and colors
    github.com/google/uuid v1.5.0              // UUID generation
)
```

### External Dependencies

- **gh CLI**: GitHub CLI must be installed and authenticated
  - Install: https://cli.github.com/
  - Authenticate: `gh auth login`

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

**Decision:** Create temporary branches (e.g., `username/stack-<name>-<uuid>`) when editing a PR.

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

## User Stories

### Story 1: Creating a Feature with 3 PRs

**As a developer**, I want to split my feature into 3 dependent PRs.

```bash
# Start from main
git checkout main
git pull
stack new auth-feature

# PR 1: Database changes
vim db/models.go
git add db/models.go
git commit -m "Add User and Session models

Creates database models for authentication.
Includes migrations for users and sessions tables.
"

# PR 2: API endpoints
vim api/auth.go
git add api/auth.go
git commit -m "Add authentication endpoints

Implements /login and /logout endpoints.
Uses JWT for session management.
"

# PR 3: Frontend integration
vim frontend/auth.js
git add frontend/auth.js
git commit -m "Add login UI

Creates login form and integrates with auth API.
Handles token storage and refresh.
"

# Push to GitHub
stack push --ready
# Creates 3 PRs: #1 → #2 → #3
```

**Result:** 3 PRs on GitHub, each based on the previous one.

---

### Story 2: Updating Middle PR After Review

**As a developer**, I want to address review comments on PR #2 without affecting PR #3.

```bash
# Select PR to edit
stack edit
# Choose: 2

# Make changes
vim api/auth.go
git add api/auth.go
git commit --amend

# Automatically rebases PR #3
# Output: ✓ Updated PR #2 and rebased 1 subsequent PR

# Push updates
stack push
# Updates PR #2 and PR #3 on GitHub
```

---

### Story 3: Inserting a New PR in the Middle

**As a developer**, I realize I need a new PR between #1 and #2.

```bash
# Edit PR #1
stack edit 1

# Create new change
vim middleware/validate.go
git add middleware/validate.go
git commit -m "Add validation middleware

Middleware to validate request parameters.
Used by auth endpoints.
"

# Automatically inserts after PR #1, shifts #2 and #3
# Output: ✓ Inserted new PR after #1, rebased 2 subsequent PRs

# Return to stack
git checkout username/stack-auth-feature

# Push
stack push
# Creates new PR #1235 between #1 and #2
```

**Result:** Stack now has 4 PRs: #1 → new PR → #2 → #3

---

### Story 4: Handling Merged PRs

**As a developer**, I want to clean up after PR #1 merges.

```bash
# PR #1 merged on GitHub
stack refresh

# Output:
# ✓ PR #1234 merged to main
# ✓ Rebasing remaining PRs on origin/main
# ✓ Deleted branch username/stack-auth-feature-550e8400

stack show
# Now shows:
# 1. Add authentication endpoints (was #2)
# 2. Add login UI (was #3)

# Push updates
stack push
# Updates PR bases: #1235 now based on main, #1236 based on #1235
```

---

### Story 5: Working Across Multiple Stacks

**As a developer**, I want to work on multiple feature stacks simultaneously.

```bash
# Create first stack
stack new feature-a
# ... make commits ...
stack push

# Create second stack
stack new feature-b
# ... make commits ...
stack push

# Switch between stacks
stack switch
# Interactive fuzzy finder shows both stacks

# Or direct switch
stack switch feature-a

# View all stacks
stack list
# * feature-a  (3 PRs, base: main)
#   feature-b  (2 PRs, base: main)
```

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

---

## Contributing

(To be added in Phase 6)

---

## License

(To be determined)
