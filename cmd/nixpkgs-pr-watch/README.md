# nixpkgs-pr-watch

Filter NixOS/nixpkgs pull requests based on packages and configured services in your NixOS configurations.

## Overview

`nixpkgs-pr-watch` analyzes your NixOS/home-manager configurations to extract dependencies, then filters nixpkgs pull requests to show only those relevant to your system. This helps you track updates and security fixes for packages you actually use.

## Features

- **Dependency Extraction**: Automatically extracts packages from `environment.systemPackages` and `home.packages`
- **Smart Matching**: Matches PRs based on:
  - File paths (high confidence): `pkgs/by-name/gi/git/package.nix` → git
  - PR titles (medium confidence): "git: 2.43.0 -> 2.44.0" → git
  - Module paths (high confidence): NixOS services inferred from configured systemd services
- **Confidence Scoring**: Filter by confidence level (high, medium, low)
- **Status Highlighting**: PRs with merge conflicts or build failures are visually highlighted
- **Flexible Filtering**: Filter by author, base branch, or confidence level
- **Sorting Options**: Sort by creation or update time
- **Display Modes**: Full detail or compact (2-line) output
- **Caching**: Smart incremental caching with TTL (24h for deps, 6h for PRs)
- **Multiple Output Formats**: Terminal (colored) or JSON
- **Multi-Host Support**: Analyze single host or all hosts in your flake

## Installation

### From Source

```bash
go install go.sbr.pm/x/cmd/nixpkgs-pr-watch@latest
```

### Build Locally

```bash
# From the repository root
go build -o nixpkgs-pr-watch ./cmd/nixpkgs-pr-watch

# Or with make (if available)
make build
```

## Usage

### Basic Usage

```bash
# Analyze current host
nixpkgs-pr-watch

# Analyze specific host
nixpkgs-pr-watch --host kyushu

# Analyze all hosts in flake
nixpkgs-pr-watch --all-hosts
```

### Filtering

```bash
# Only show high confidence matches
nixpkgs-pr-watch --min-confidence high

# Limit number of PRs fetched
nixpkgs-pr-watch --limit 100

# Filter by author (e.g., for r-ryantm bot updates)
nixpkgs-pr-watch --user r-ryantm

# Filter by base branch
nixpkgs-pr-watch --base-branch staging
```

### Display Options

```bash
# Compact output (2 lines per PR)
nixpkgs-pr-watch --compact

# Sort by update time instead of creation time
nixpkgs-pr-watch --sort updated
```

### Output Formats

```bash
# Terminal output (default)
nixpkgs-pr-watch

# JSON output
nixpkgs-pr-watch --output json

# JSON with jq filtering
nixpkgs-pr-watch --output json | jq '.matches[] | select(.score > 80)'
```

### Cache Management

```bash
# Refresh dependency cache
nixpkgs-pr-watch --refresh-deps

# Refresh PR cache
nixpkgs-pr-watch --refresh-prs

# Refresh both
nixpkgs-pr-watch --refresh

# Clear all caches
nixpkgs-pr-watch cache clear

# Show cache info
nixpkgs-pr-watch cache info
```

## Example Output

### Full Output (default)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ NixOS/nixpkgs PRs matching your configuration                              │
│ Analyzed: kyushu (392 packages, 0 modules)                                │
│ Found: 2 relevant PRs                                                        │
└─────────────────────────────────────────────────────────────────────────────┘

HIGH CONFIDENCE MATCHES (2)
════════════════════════════════════════════════════════════════════════════════

[#479757] oci-cli: 3.71.4 -> 3.72.0
  → Matches: oci-cli (package)
  │ Files: pkgs/by-name/oc/oci-cli/package.nix (+2/-2)
  │ Labels: 10.rebuild-linux: 1-10, merge-bot eligible
  │ Created: 2d ago | Updated: 1d ago
  │ Author: @r-ryantm
  └ https://github.com/NixOS/nixpkgs/pull/479757

[#479713] GNOME updates 2026-01-13 ⚠️  CONFLICTS
  → Matches: nautilus (package)
  │ Files: pkgs/by-name/eo/eog/package.nix and 4 more files
  │ Labels: 10.rebuild-linux: 11-100
  │ Created: 3d ago | Updated: 2d ago
  │ Author: @bobby285271
  └ https://github.com/NixOS/nixpkgs/pull/479713
```

### Compact Output (`--compact`)

```
[#479757] oci-cli: 3.71.4 -> 3.72.0 (created: 2d ago)
  📦 oci-cli (package) by @r-ryantm - https://github.com/NixOS/nixpkgs/pull/479757

[#479713] GNOME updates 2026-01-13 (created: 3d ago) ⚠️  CONFLICTS
  📦 nautilus (package) by @bobby285271 - https://github.com/NixOS/nixpkgs/pull/479713
```

## How It Works

1. **Dependency Extraction**:
   - Uses `nix eval` to extract package names from your NixOS configuration
   - Extracts from both `environment.systemPackages` and `home-manager` packages
   - Caches results for 24 hours (invalidates on flake.lock changes)

2. **PR Fetching**:
   - Uses `gh` CLI to fetch open PRs from NixOS/nixpkgs
   - Caches results for 6 hours
   - Fetches PR metadata including files changed, labels, author

3. **Matching Algorithm**:
   - **High confidence**: File path matches package name (`pkgs/by-name/gi/git/package.nix` → git)
   - **Medium confidence**: PR title contains package name ("git: 2.43.0 -> 2.44.0" → git)
   - **Module paths**: Derived from configured systemd services (e.g., `nixos/modules/services/docker`)
   - **Low confidence**: Heuristic matches (currently disabled)

4. **Scoring**:
   - High confidence: +100 points
   - Medium confidence: +50 points
   - Score capped at 100

## Development

### Project Structure

Following Go best practices:

```
cmd/nixpkgs-pr-watch/  # Main application entry point
├── main.go            # Root command and CLI setup
├── watch.go           # Main watch logic
└── cache.go           # Cache management commands

internal/              # Private packages (not importable externally)
├── cache/             # TTL-based caching
│   ├── cache.go
│   └── cache_test.go
├── config/            # Flake configuration
│   └── config.go
├── deps/              # Dependency extraction
│   ├── deps.go
│   └── deps_test.go
├── output/            # Terminal output formatting
│   └── output.go
└── pr/                # PR fetching and matching
    ├── types.go
    ├── fetcher.go
    ├── matcher.go
    └── matcher_test.go
```

### Building

```bash
# Build for current platform
go build -o nixpkgs-pr-watch ./cmd/nixpkgs-pr-watch

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o nixpkgs-pr-watch-linux ./cmd/nixpkgs-pr-watch

# Install to $GOPATH/bin
go install ./cmd/nixpkgs-pr-watch
```

### Testing

Run tests following TDD best practices:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detector
go test -race ./...

# Run specific package tests
go test ./internal/pr/...
go test ./internal/cache/...

# Run specific test
go test -run TestMatcher_matchPR ./internal/pr/...
```

### Code Quality

```bash
# Format code (always run before committing)
gofmt -w .
go fmt ./...

# Run linter
go vet ./...

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify
```

### Test Coverage

Current coverage:
- `internal/cache`: 83.6%
- `internal/pr`: 71.0%
- `internal/deps`: 46.2% (lower due to nix integration, tested via integration)

### Writing Tests

Follow table-driven test pattern:

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {
            name: "descriptive test case name",
            input: "test input",
            want: "expected output",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := DoSomething(tt.input)
            if got != tt.want {
                t.Errorf("DoSomething(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## Requirements

- Go 1.21+ (for building)
- `nix` CLI (for dependency extraction)
- `gh` CLI (for PR fetching)
- A NixOS flake with `nixosConfigurations`

## Configuration

The tool detects your flake automatically from the current directory or can be specified:

```bash
nixpkgs-pr-watch --flake /path/to/nixos/config
```

## Caching

Caches are stored in `~/.cache/nixpkgs-pr-watch/`:
- `<hostname>-deps.json`: Dependency cache (TTL: 24h)
- `nixpkgs-prs-data.json`: PR cache data (TTL: 6h)
- `nixpkgs-prs-metadata.json`: PR cache metadata (TTL: 6h)

## Limitations

- Currently only extracts `environment.systemPackages` and `home.packages`
- Module detection is inferred from systemd services; home-manager modules not yet extracted
- Doesn't detect transitive dependencies
- Title matching can have false positives with short package names

## Roadmap

### Phase 1 (MVP) ✅
- [x] Basic dependency extraction
- [x] PR fetching via gh CLI
- [x] Package name matching
- [x] Terminal output
- [x] JSON export
- [x] Caching

### Phase 2
- [ ] Home-manager module extraction
- [ ] Enhanced confidence scoring
- [ ] Better title matching (word boundaries)

### Phase 3
- [ ] Label filtering
- [ ] Configuration file support
- [ ] Interactive mode (fzf)

### Future
- [ ] Watch mode (continuous monitoring)
- [ ] Notifications (ntfy integration)
- [ ] Web dashboard
- [ ] Impact analysis (rebuild estimates)

## Contributing

Contributions are welcome! Please follow these guidelines:

### Development Workflow

1. **Test-Driven Development**: Write tests first, then implement features
   - RED: Write a failing test
   - GREEN: Write minimal code to pass
   - REFACTOR: Improve code while keeping tests green

2. **Code Quality**:
   - Run `go fmt ./...` before committing
   - Run `go vet ./...` to catch common mistakes
   - Ensure all tests pass: `go test ./...`
   - Maintain or improve test coverage

3. **Commit Messages**:
   - Use conventional commits format
   - Examples: `feat: add label filtering`, `fix: handle empty PR list`, `test: add matcher edge cases`

### Adding New Features

1. Write tests first (table-driven tests preferred)
2. Implement the feature
3. Ensure all tests pass
4. Update documentation
5. Submit PR with clear description

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Keep functions small and focused
- Use meaningful variable names
- Add comments for exported functions
- Prefer composition over inheritance
- Handle errors explicitly

## License

MIT

## Author

Vincent Demeester
