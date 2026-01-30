package lazypr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	// When no config file exists, should return default config
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Default config should have empty actions
	if len(cfg.Actions) != 0 {
		t.Errorf("Default config should have 0 actions, got %d", len(cfg.Actions))
	}
}

func TestLoadConfig_WithActions(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[[actions]]
name = "Open in browser"
command = "xdg-open {url}"
interactive = false

[[actions]]
name = "Review with Claude"
command = "claude review {url}"
interactive = true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.Actions) != 2 {
		t.Fatalf("Expected 2 actions, got %d", len(cfg.Actions))
	}

	// Check first action
	if cfg.Actions[0].Name != "Open in browser" {
		t.Errorf("Action[0].Name = %q, want %q", cfg.Actions[0].Name, "Open in browser")
	}
	if cfg.Actions[0].Command != "xdg-open {url}" {
		t.Errorf("Action[0].Command = %q, want %q", cfg.Actions[0].Command, "xdg-open {url}")
	}
	if cfg.Actions[0].Interactive != false {
		t.Errorf("Action[0].Interactive = %v, want false", cfg.Actions[0].Interactive)
	}

	// Check second action
	if cfg.Actions[1].Name != "Review with Claude" {
		t.Errorf("Action[1].Name = %q, want %q", cfg.Actions[1].Name, "Review with Claude")
	}
	if cfg.Actions[1].Interactive != true {
		t.Errorf("Action[1].Interactive = %v, want true", cfg.Actions[1].Interactive)
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	pr := PRDetail{
		Number: 123,
		Title:  "Fix bug in parser",
		Author: "testuser",
		Owner:  "NixOS",
		Repo:   "nixpkgs",
		URL:    "https://github.com/NixOS/nixpkgs/pull/123",
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "url placeholder",
			command:  "xdg-open {url}",
			expected: "xdg-open 'https://github.com/NixOS/nixpkgs/pull/123'",
		},
		{
			name:     "number placeholder",
			command:  "gh pr view {number}",
			expected: "gh pr view '123'",
		},
		{
			name:     "full_name placeholder",
			command:  "cd $(repo-find {full_name})",
			expected: "cd $(repo-find 'NixOS/nixpkgs')",
		},
		{
			name:     "multiple placeholders",
			command:  "echo {owner}/{repo}#{number}",
			expected: "echo 'NixOS'/'nixpkgs'#'123'",
		},
		{
			name:     "title with special chars",
			command:  "echo {title}",
			expected: "echo 'Fix bug in parser'",
		},
		{
			name:     "author placeholder",
			command:  "gh api users/{author}",
			expected: "gh api users/'testuser'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstitutePlaceholders(tt.command, pr)
			if got != tt.expected {
				t.Errorf("SubstitutePlaceholders() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSubstitutePlaceholders_ShellEscape(t *testing.T) {
	// Test that special characters are properly escaped
	pr := PRDetail{
		Number: 123,
		Title:  "Fix 'quoted' and \"double\" chars",
		Author: "test",
		Owner:  "owner",
		Repo:   "repo",
		URL:    "https://example.com",
	}

	got := SubstitutePlaceholders("echo {title}", pr)
	// Single quotes in the value should be escaped
	if got != "echo 'Fix '\\''quoted'\\'' and \"double\" chars'" {
		t.Errorf("Shell escaping failed: got %q", got)
	}
}

func TestSubstituteBatchPlaceholders(t *testing.T) {
	prs := []PRDetail{
		{
			Number: 123,
			Title:  "First PR",
			Author: "user1",
			Owner:  "NixOS",
			Repo:   "nixpkgs",
			URL:    "https://github.com/NixOS/nixpkgs/pull/123",
		},
		{
			Number: 456,
			Title:  "Second PR",
			Author: "user2",
			Owner:  "NixOS",
			Repo:   "nixpkgs",
			URL:    "https://github.com/NixOS/nixpkgs/pull/456",
		},
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "urls placeholder",
			command:  "lazypr {urls}",
			expected: "lazypr 'https://github.com/NixOS/nixpkgs/pull/123' 'https://github.com/NixOS/nixpkgs/pull/456'",
		},
		{
			name:     "numbers placeholder",
			command:  "gh pr view {numbers}",
			expected: "gh pr view '123' '456'",
		},
		{
			name:     "mixed singular and plural",
			command:  "echo {owner} && firefox {urls}",
			expected: "echo 'NixOS' && firefox 'https://github.com/NixOS/nixpkgs/pull/123' 'https://github.com/NixOS/nixpkgs/pull/456'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteBatchPlaceholders(tt.command, prs)
			if got != tt.expected {
				t.Errorf("SubstituteBatchPlaceholders() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSubstituteBatchPlaceholders_SinglePR(t *testing.T) {
	prs := []PRDetail{
		{
			Number: 123,
			Title:  "Only PR",
			Author: "user1",
			Owner:  "NixOS",
			Repo:   "nixpkgs",
			URL:    "https://github.com/NixOS/nixpkgs/pull/123",
		},
	}

	got := SubstituteBatchPlaceholders("lazypr {urls}", prs)
	expected := "lazypr 'https://github.com/NixOS/nixpkgs/pull/123'"
	if got != expected {
		t.Errorf("SubstituteBatchPlaceholders() = %q, want %q", got, expected)
	}
}
