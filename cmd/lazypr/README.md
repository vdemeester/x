# lazypr

TUI for viewing GitHub pull requests with optional filtering and multi-PR input.

## Usage

```bash
lazypr [ref] [ref...]
```

When run without arguments in a git repository with a GitHub remote, it auto-detects the repository and loads open PRs.

Accepted references:
- GitHub URL: `https://github.com/owner/repo/pull/123`
- PR ref: `owner/repo#123`
- Repo ref: `owner/repo` (loads open PRs from the repo)

## Examples

```bash
# Auto-detect repository from git remote
lazypr

# Load open PRs from a repo
lazypr tektoncd/operator

# Load specific PRs
lazypr owner/repo#1 owner/repo#2

# Filter PRs by label and author
lazypr -L bug -L "needs-review" -a octocat

# Show all PRs (open + closed)
lazypr -s all
```

## Options

- `-l, --limit`: Maximum number of PRs to fetch when loading from a repo (default: 100)
- `-L, --label`: Filter by label (can be repeated)
- `-m, --milestone`: Filter by milestone
- `-a, --author`: Filter by author
- `-s, --state`: Filter by state (open, closed, all)

## Building

```bash
go build -o lazypr ./cmd/lazypr
```

## Installing

```bash
go install go.sbr.pm/x/cmd/lazypr@latest
```
