# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is **Stack**, a git-native CLI tool for managing stacked pull requests on GitHub. It uses standard git primitives (branches, commits, rebase) and follows a Gerrit-inspired workflow where each commit represents one pull request. The tool integrates with GitHub via the `gh` CLI.

## Building and Testing

### Build the project
```bash
go build -o stack .
```

### Install locally
```bash
go install
```

### Run the CLI
```bash
stack <command>
```

### Format code
```bash
goimports -w .
go fmt ./...
```

### Build and test in one command
```bash
go build && ./stack list
```

## Architecture

### Core Design Principles

1. **Git-native approach**: All operations use standard git primitives. Users can use regular git commands alongside the tool.
2. **One commit = One PR**: Each commit on a stack branch represents exactly one pull request, inspired by Gerrit.
3. **Git trailers for metadata**: PR metadata (UUID, stack name) is stored as git trailers in commit messages, ensuring metadata travels with commits during rebases.
4. **Bottom-up merging**: PRs must merge from bottom to top for predictability.
5. **UUID branches for editing**: Temporary branches (e.g., `username/stack-<name>/<uuid>`) are created when editing a specific PR in the middle of a stack.

### Key Components

**Git Client** (`internal/git/client.go`)
- Central git operations wrapper using `exec.Command`
- All git operations are delegated through this client for consistency
- Dependency injection pattern: Commands receive `*git.Client` to enable testing

**Domain Models** (`internal/model/`)
- `Stack` - Stack configuration with base branch, timestamps, sync tracking, and merged changes
- `Change` - Individual PR/commit with position tracking (absolute and active)
- `PR` - GitHub PR metadata with versioning, draft status tracking, and cached metadata
- `PRData` - Versioned wrapper for PR tracking (currently version 1)
- Separate package for clean domain model separation

**Stack Client** (`internal/stack/client.go`)
- Large orchestration layer managing all stack operations
- Manages stack metadata stored in `.git/stack/<stack-name>/`
- Each stack has `config.json` (stack metadata) and `prs.json` (PR tracking with versioning)
- Provides `GetStackContext()` to determine current stack from branch name
- `GetStackContextByName(name)` loads a specific stack's context by name
- Methods: `LoadPRs()`, `SavePRs()` work with versioned PR data
- Mutations now go through `StackContext.Save()` which persists both PRs and Stack metadata
- Handles sync status checking (5-minute staleness threshold)

**Stack Context** (`internal/stack/context.go`)
- `StackContext` is the primary abstraction for working with stacks
- Contains stack metadata, all changes, and editing state
- Key methods:
  - `IsStack()` - returns true if context represents a stack
  - `IsEditing()` - returns true if editing a specific change (on UUID branch)
  - `CurrentChange()` - returns the change being edited (or nil)
  - `FindChange(uuid)` - finds a change by UUID in the stack
  - `FormatUUIDBranch(username, uuid)` - formats a UUID branch name
- Also provides branch helper functions: `IsUUIDBranch()`, `ExtractStackName()`, `ExtractUUIDFromBranch()`, `FormatStackBranch()`

**Command Pattern** (`cmd/command.go`)
- Each command implements the `Command` interface with a `Register()` method
- Commands are registered in `cmd/root.go` init()
- Each command struct holds its own clients (`Git` and `Stack`) for dependency injection

**UI System** (`internal/ui/`) - 11 files
- Centralized terminal styling and formatting using `lipgloss`
- `config.go` - UI configuration settings
- `format.go` - Reusable formatting utilities (truncate, pad, boxes, panels)
- `styles.go` - Consistent color scheme and style definitions
- `render.go` - Stack rendering (list view, details view, push progress)
- `status.go` - Status rendering for stack details
- `select.go` - Fuzzy finder for interactive change selection
- `table.go` - Table formatting for stack display
- `tree.go` - Tree-based stack visualization
- `input.go` - User input handling
- `print.go` - Simple output functions
- `terminal.go` - Terminal utilities and width detection
- All commands use the UI system for consistent output

**GitHub Client** (`internal/gh/client.go`)
- Wraps `gh` CLI for all GitHub operations
- `SyncPR()` - Idempotent PR creation/update with auto-recovery
- `BatchGetPRs()` - Efficient batch PR queries via GraphQL
- `GetPRState()` - Query individual PR merge status
- `MarkPRReady()` / `MarkPRDraft()` - Toggle PR draft status
- `ListPRComments()` / `CreatePRComment()` / `UpdatePRComment()` - Comment management for stack visualization
- `OpenPR()` - Open PR in browser

**Branch Naming Conventions**
- Stack branch: `username/stack-<name>/TOP` (e.g., `bjulian5/stack-auth-refactor/TOP`)
- UUID branch: `username/stack-<name>/<uuid>` (e.g., `bjulian5/stack-auth-refactor/550e8400`)
- Helper functions in `internal/stack/context.go` for parsing and formatting
- The `/TOP` suffix represents the top of the stack (the working branch with all commits)

**Metadata Storage**
- `.git/stack/<stack-name>/config.json`: Stack configuration (name, branch, base, timestamps)
- `.git/stack/<stack-name>/prs.json`: PR tracking (maps UUID to PR number, URL, state, commit hash)
- Current stack is determined by branch context (via `GetStackContext()`), not stored in a file

**Commit Message Structure**
```
Add JWT authentication                    ← PR title (first line)
                                          ← blank line
Implements secure JWT-based auth with    ← PR description (body)
refresh tokens and cookie handling.

PR-UUID: 550e8400-e29b-41d4-a716
PR-Stack: auth-refactor
```

**Commit Data Structures**
The codebase uses structured types for commit parsing:
- `git.Commit` - represents a commit with `Hash` (string) and `Message` (`CommitMessage`)
- `git.CommitMessage` - parsed message with `Title` (string), `Body` (string), and `Trailers` (map)
- `ParseCommitMessage(message string)` - parses raw commit message into structured form
- `CommitMessage.AddTrailer(key, value)` - adds a trailer
- `CommitMessage.String()` - converts back to formatted commit message string

### Code Organization

```
stack/
├── main.go                          # Entry point, calls cmd.Execute()
├── cmd/
│   ├── root.go                      # Root command setup with cobra
│   ├── command.go                   # Command interface
│   ├── install/install.go           # stack install command
│   ├── newcmd/new.go                # stack new command (newcmd to avoid "new" keyword)
│   ├── list/list.go                 # stack list command
│   ├── status/status.go             # stack status command
│   ├── edit/edit.go                 # stack edit command (interactive fuzzy finder only)
│   ├── fixup/fixup.go               # stack fixup command
│   ├── switch/switch.go             # stack switch command (package: switchcmd)
│   ├── top/top.go                   # stack top command
│   ├── bottom/bottom.go             # stack bottom command
│   ├── up/up.go                     # stack up command
│   ├── down/down.go                 # stack down command
│   ├── push/push.go                 # stack push command (--dry-run, --force flags)
│   ├── refresh/refresh.go           # stack refresh command
│   ├── restack/restack.go           # stack restack command
│   ├── delete/delete.go             # stack delete command
│   ├── cleanup/cleanup.go           # stack cleanup command
│   ├── pr/
│   │   ├── pr.go                    # Parent PR command
│   │   ├── open/open.go             # stack pr open command
│   │   ├── ready/ready.go           # stack pr ready command (--all flag)
│   │   └── draft/draft.go           # stack pr draft command (--all flag)
│   └── hook/
│       ├── hook.go                  # Parent hook command
│       ├── prepare_commit_msg.go    # prepare-commit-msg hook implementation
│       ├── commit_msg.go            # commit-msg hook implementation
│       ├── post_commit.go           # post-commit hook implementation
│       └── operations.go            # Common hook operations and workflows
├── internal/
│   ├── git/
│   │   ├── client.go                # Core git operations wrapper
│   │   ├── commit.go                # Commit and CommitMessage types with parsing
│   │   ├── rebase.go                # Rebase operations for stack updates
│   │   └── template.go              # Commit message templates
│   ├── model/
│   │   ├── stack.go                 # Stack domain model
│   │   ├── change.go                # Change domain model
│   │   └── pr.go                    # PR and PRData models with versioning
│   ├── stack/
│   │   ├── client.go                # Stack metadata management (1385 lines - core orchestration)
│   │   ├── config.go                # Stack and global configuration
│   │   ├── context.go               # StackContext for branch-based state and branch helpers
│   │   ├── visualization.go         # Stack visualization in PR comments
│   │   └── rebase_state.go          # Rebase state management for recovery
│   ├── gh/
│   │   ├── client.go                # GitHub client via gh CLI
│   │   └── types.go                 # GitHub types (PRSpec, Comment)
│   ├── ui/
│   │   ├── config.go                # UI configuration settings
│   │   ├── format.go                # Formatting utilities and helper functions
│   │   ├── styles.go                # lipgloss style definitions
│   │   ├── render.go                # Stack rendering functions
│   │   ├── status.go                # Status rendering
│   │   ├── select.go                # Interactive fuzzy finder
│   │   ├── table.go                 # Table formatting
│   │   ├── tree.go                  # Tree-based stack visualization
│   │   ├── input.go                 # User input handling
│   │   ├── print.go                 # Simple output functions
│   │   └── terminal.go              # Terminal utilities
│   ├── hooks/
│   │   └── install.go               # Hook installation/uninstallation
│   └── common/
│       └── utils.go                 # Shared utilities (username detection, UUID generation, etc.)
```

## Implementation Status

The codebase has completed **Phase 1** (Foundation), **Phase 2** (Git Hooks), **Phase 3** (Editing & Navigation), **Phase 4** (GitHub Integration), and **Phase 5** (Sync & Refresh):

**Phase 1 - Foundation (✅ Completed):**
- ✅ `stack new <name>` - Create new stack
- ✅ `stack list` - List all stacks
- ✅ `stack status [name]` - Show stack status
- ✅ Core git operations (branch management, commit parsing)
- ✅ Stack metadata storage and retrieval

**Phase 2 - Git Hooks (✅ Completed):**
- ✅ `prepare-commit-msg` hook - Automatic UUID and trailer injection
- ✅ `post-commit` hook - Stack updates after commits on UUID branches
- ✅ `commit-msg` hook - Commit message validation
- ✅ Hook installation/uninstallation
- ✅ Amend and insert operations for stack editing

**Phase 3 - Editing & Navigation (✅ Completed):**
- ✅ `stack edit` - Interactive PR editing with fuzzy finder (no direct arguments)
- ✅ `stack switch [name]` - Stack switching with fuzzy finder
- ✅ `stack top/bottom/up/down` - Navigate through stack changes
- ✅ `stack delete [name]` - Delete stacks with archival
- ✅ `stack cleanup` - Clean up fully merged or empty stacks
- ✅ UI system with lipgloss for styled terminal output (11 files)
- ✅ Tree-based and table-based rendering options
- ✅ Uncommitted changes validation before operations

**Phase 4 - GitHub Integration (✅ Completed):**
- ✅ `stack push` - Push PRs to GitHub (--dry-run, --force flags)
- ✅ `stack pr ready/draft` - Mark PRs as ready or draft (--all flag)
- ✅ `stack install` - Install hooks and configure git
- ✅ `stack pr open` - Open PRs in browser (--select flag)
- ✅ GitHub client with batch API queries
- ✅ Stack visualization in PR comments with caching
- ✅ Idempotent PR sync (create or update)
- ✅ Draft status tracking (local vs remote)

**Phase 5 - Sync & Refresh (✅ Completed):**
- ✅ `stack refresh` - Detect and handle merged PRs
- ✅ `stack restack` - Rebase on base branch with recovery system
- ✅ `stack fixup` - Interactive fixup commits with autosquash
- ✅ Rebase state management for conflict recovery
- ✅ Bottom-up merge validation

## Development Patterns

### Adding a New Command

1. Create a new package under `cmd/<command-name>/`
2. Implement the `Command` interface:
   ```go
   type Command struct {
       Git   *git.Client
       Stack *stack.Client
       // flags and arguments
   }

   func (c *Command) Register(parent *cobra.Command) {
       // Initialize clients
       // Create cobra command
       // Register flags
       // Add to parent
   }
   ```
3. Register in `cmd/root.go` init()

### Dependency Injection Pattern

All commands use dependency injection for git and stack clients:
```go
func (c *Command) Register(parent *cobra.Command) {
    c.Git, err = git.NewClient()
    c.Stack = stack.NewClient(c.Git.GitRoot())
    // ...
}
```

This enables:
- Easy mocking in tests
- Consistent client initialization
- Clear dependencies

### Git Operations

Always use `git.Client` methods instead of calling git directly:
- `c.Git.GetCurrentBranch()` - get the current branch name
- `c.Git.GetCommit(hash)` - get a commit with parsed message (includes `ShortHash()` method)
- `c.Git.GetCommits(branch, base)` - get all commits between base and branch
- `c.Git.CheckoutBranch(name)` / `c.Git.CreateAndCheckoutBranch(name)` - branch operations
- `c.Git.CreateAndCheckoutBranchAt(name, commitHash)` - create branch at specific commit (used by `stack edit`)
- `c.Git.HasUncommittedChanges()` - check for uncommitted changes before operations
- `c.Git.RebaseSubsequentCommits(...)` - rebase commits after a stack update
- All git operations go through the client for consistency and testability

Note: The git client API has been simplified - many unused methods were removed in favor of focused operations that support the core stack workflows.

### Error Handling

Return descriptive errors with context:
```go
if err != nil {
    return fmt.Errorf("failed to <operation>: %w", err)
}
```

## Key Files to Reference

- **go.mod**: Dependencies include cobra (CLI), go-git (git operations), go-fuzzyfinder (interactive selection), lipgloss (terminal styling)

## Notes on Naming

- The `newcmd` package is named this way (not just `new`) because `new` is a Go keyword
- Branch parsing functions handle both stack branches and UUID branches
- Username detection happens in `internal/common/utils.go` (checks git config, gh config)
- Remeber to use `fd` instead of `find` since `fd` is much faster and more ergonomic.
