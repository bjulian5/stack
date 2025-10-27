# Stack

`stack` is a CLI tool that allows you to manage a stack of pull requests in a GitHub repository where a stack is a series of dependent pull requests that build on top of each other. `stack` is inspired by `gerrit` and aims to minimize the cognitive overhead of switching mental models between Git, Github and stacking abstractions by treating each commit as a pull request in a stack. A very simple workflow is as follows:

1. Create a new stack of changes by running `stack new <stack-name>`. This will create a new stack using your current branch as the base.
2. Add commits to the stack just as you would normally using `git commit`. Each commit is a new pull request in the stack and the title and description of the pull request is taken from the commit message.
3. Push the stack to GitHub using `stack push`. This will create a series of pull requests on GitHub, each corresponding to a commit in the stack.
4. As you receive feedback and make changes, amend commits using `git commit --amend` then run `stack push` again to update the corresponding pull requests on GitHub.


# Installation
You can install `stack` by running `go install github.com/bjulian5/stack@latest`. Make sure that your `GOPATH/bin` is in your `PATH` so that you can run the `stack` command from anywhere.

`stack` relies on git hooks to manage the stack state. You can install the necessary git hooks and dependencies by running
```
stack install
```

## Shell Completion
Bash completion can be enabled by running
```bash
source <(stack completion zsh)  # for zsh
source <(stack completion bash) # for bash
```

# Basic Usage
```help
$ stack --help
Stack is a CLI tool for managing stacked pull requests on GitHub.

It allows developers to create, manage, and sync stacked PRs while using
familiar git commands for most operations.

Usage:
  stack [flags]
  stack [command]

Available Commands:
  bottom      Move to the bottom of the stack
  cleanup     Clean up stacks with all PRs merged or empty stacks
  completion  Generate the autocompletion script for the specified shell
  delete      Delete a stack and its branches
  down        Move down to the previous change in the stack
  edit        Edit a change in the stack
  fixup       Create a fixup commit for a change in the stack
  help        Help about any command
  install     Install stack hooks and configure git
  list        List all stacks
  new         Create a new stack
  pr          PR operations
  push        Push PRs to GitHub
  refresh     Sync stack with GitHub to detect merged PRs
  restack     Rebase the stack on top of the latest base branch changes
  status      Show status of a stack
  switch      Switch to a different stack
  top         Move to the top of the stack
  up          Move up to the next change in the stack

Flags:
  -h, --help   help for stack
```

## Creating a new stack
To create a new stack, run
```
stack new <stack-name> {--base <base-branch>}
```
to create a new stack using your current branch as the base. You can optionally specify a different base branch using the `--base` flag.

## Adding to your stack
To add changes to your stack, simply create new commits using `git commit`. Each commit will be treated as a new pull request in the stack.

## Navigating your stack
To view the current state of your stack, run
```
stack status                # Status of the current stack
stack status auth-refactor  # Status of a specific stack
```

You can then navigate around your stack using any of the following
```
stack top        # Move to the top of the stack
stack bottom     # Move to the bottom of the stack
stack up         # Move up one change in the stack
stack down       # Move down one change in the stack
stack edit       # Open an interactive picker to move to a specific change in the stack
```

## Moving to another stack
You can list the available stacks in your repository by running
```
stack list
```

To move to another stack, run
```
stack switch                  # Interactive fuzzy finder
stack switch auth-refactor    # Direct switch
```

## Editing a Change in the stack
To edit a change in the stack, normal git commands can be used. To simplify things, you can run
```bash
git commit --amend {--no-edit} # --no-edit skips modifying the commit message
```
to edit the most recent change in the stack, including modifying the pull request title and description.

To edit a different change in the stack, navigate to that change using any of the above navigation commands, or run
```
stack edit
```
to directly navigate to the change you want to edit. Once you are on the desired change, run
```bash
git commit --amend {--no-edit}
```

After making your change, you can return to the top of your stack by running
```
stack top
```
A useful git alias to simplify this workflow is
```gitconfig
[alias]
	fixup = !sh -c 'REV=$(git rev-parse $1) && git commit --fixup $@ && GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash $REV^' -
```
which allows you to run `git fixup <commitlike>` to automatically fixup the specified commit.

`stack` also includes a native workflow for this as well with `stack fixup` which opens an interactive picker to select the change you want to fixup.

## Inserting a new change in the stack
Since each commit corresponds to a pull request in the stack, you can insert a new change anywhere in the stack by navigating to the change and adding a new commit.

```bash
stack down                     # Navigate to the desired change
git commit -m "New change"     # Add a new commit
stack top                      # Return to the top of the stack
```

## Pushing your stack to GitHub
To push your stack to GitHub, run
```
stack push
```
to sync your local stack with GitHub. This will create or update the corresponding pull requests on GitHub.

### Pulling in upstream changes
When a change is merged, you can run
```bash
stack refresh
```
to automatically fetch the latest updates from the base branch and rebase your stack on top of the latest changes.

## Marking a change as draft/ready
```
stack pr {ready|draft} # Mark the current change as ready or draft
```

## Opening pull requests in the browser
To open the pull requests in your stack in the browser, navigate to the change and run
```
stack pr open
```

