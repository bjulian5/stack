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
5. **UUID branches for editing**: Temporary branches (e.g., `username/stack-<name>-<uuid>`) are created when editing a specific PR in the middle of a stack.

### Key Components

**Git Client** (`internal/git/client.go`)
- Central git operations wrapper using `exec.Command`
- All git operations are delegated through this client for consistency
- Dependency injection pattern: Commands receive `*git.Client` to enable testing

**Stack Client** (`internal/stack/client.go`)
- Manages stack metadata stored in `.git/stack/<stack-name>/`
- Each stack has `config.json` (stack metadata) and `prs.json` (PR tracking)
- Handles current stack state (`.git/stack/current`)

**Command Pattern** (`cmd/command.go`)
- Each command implements the `Command` interface with a `Register()` method
- Commands are registered in `cmd/root.go` init()
- Each command struct holds its own clients (`Git` and `Stack`) for dependency injection

**Branch Naming Conventions**
- Stack branch: `username/stack-<name>` (e.g., `bjulian5/stack-auth-refactor`)
- UUID branch: `username/stack-<name>-<uuid>` (e.g., `bjulian5/stack-auth-refactor-550e8400`)
- Helper functions in `internal/git/branch.go` for parsing and formatting

**Metadata Storage**
- `.git/stack/<stack-name>/config.json`: Stack configuration (name, branch, base, timestamps)
- `.git/stack/<stack-name>/prs.json`: PR tracking (maps UUID to PR number, URL, state)
- `.git/stack/current`: Current active stack name

**Commit Message Structure**
```
Add JWT authentication                    ← PR title (first line)
                                          ← blank line
Implements secure JWT-based auth with    ← PR description (body)
refresh tokens and cookie handling.

PR-UUID: 550e8400-e29b-41d4-a716
PR-Stack: auth-refactor
```

### Code Organization

```
stack/
├── main.go                          # Entry point, calls cmd.Execute()
├── cmd/
│   ├── root.go                      # Root command setup with cobra
│   ├── command.go                   # Command interface
│   ├── list/list.go                 # stack list command
│   ├── show/show.go                 # stack show command
│   └── newcmd/new.go                # stack new command (newcmd to avoid "new" keyword)
├── internal/
│   ├── git/
│   │   ├── client.go                # Core git operations wrapper
│   │   ├── branch.go                # Branch name parsing/formatting
│   │   ├── commit.go                # Commit parsing
│   │   └── operations.go            # Additional git operations
│   ├── stack/
│   │   ├── client.go                # Stack metadata management
│   │   ├── stack.go                 # Stack struct
│   │   └── pr.go                    # PR struct and PRMap type
│   └── common/
│       └── utils.go                 # Shared utilities (username detection, etc.)
```

## Implementation Status

The codebase is currently in **Phase 1** of development (see DESIGN.md for full roadmap):

**Completed:**
- ✅ `stack new <name>` - Create new stack
- ✅ `stack list` - List all stacks
- ✅ `stack show [name]` - Show stack details
- ✅ Core git operations (branch management, commit parsing)
- ✅ Stack metadata storage and retrieval

**Not Yet Implemented:**
- Git hooks (prepare-commit-msg, post-commit, commit-msg)
- `stack edit` command for editing PRs in the middle of stack
- `stack switch` command with fuzzy finder
- `stack push` command to push PRs to GitHub
- `stack refresh` command to handle merged PRs
- GitHub integration via `gh` CLI

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
- `c.Git.GetCurrentBranch()` not `exec.Command("git", "branch"...)`
- `c.Git.CreateBranch(name, hash)` not manual git commands
- All git operations go through the client for consistency

### Error Handling

Return descriptive errors with context:
```go
if err != nil {
    return fmt.Errorf("failed to <operation>: %w", err)
}
```

## Key Files to Reference

- **DESIGN.md**: Comprehensive design document with full workflow details, implementation phases, and user stories
- **go.mod**: Dependencies include cobra (CLI), go-git (git operations), go-fuzzyfinder (interactive selection), lipgloss (terminal styling)

## Notes on Naming

- The `newcmd` package is named this way (not just `new`) because `new` is a Go keyword
- Branch parsing functions handle both stack branches and UUID branches
- Username detection happens in `internal/common/utils.go` (checks git config, gh config)
