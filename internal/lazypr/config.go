package lazypr

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Action represents a custom action that can be executed on a PR.
type Action struct {
	Name        string `toml:"name"`
	Command     string `toml:"command"`
	Interactive bool   `toml:"interactive"`
}

// Config holds the lazypr configuration.
type Config struct {
	Actions []Action `toml:"actions"`
}

// LoadConfig loads configuration from the given path.
// If the file doesn't exist, returns a default config.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Actions: []Action{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = home + "/.config"
	}
	return configDir + "/lazypr/config.toml"
}

// SubstitutePlaceholders replaces placeholders in command with PR data.
// Placeholders: {url}, {number}, {title}, {author}, {owner}, {repo}, {full_name}
func SubstitutePlaceholders(command string, pr PRDetail) string {
	replacements := map[string]string{
		"{url}":       shellEscape(pr.URL),
		"{number}":    shellEscape(fmt.Sprintf("%d", pr.Number)),
		"{title}":     shellEscape(pr.Title),
		"{author}":    shellEscape(pr.Author),
		"{owner}":     shellEscape(pr.Owner),
		"{repo}":      shellEscape(pr.Repo),
		"{full_name}": shellEscape(fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)),
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// SubstituteBatchPlaceholders replaces placeholders for multiple PRs.
// Plural placeholders: {urls}, {numbers}, {titles}, {authors}
// Singular placeholders use the first PR's data.
func SubstituteBatchPlaceholders(command string, prs []PRDetail) string {
	if len(prs) == 0 {
		return command
	}

	// Collect plural values
	var urls, numbers, titles, authors []string
	for _, pr := range prs {
		urls = append(urls, shellEscape(pr.URL))
		numbers = append(numbers, shellEscape(fmt.Sprintf("%d", pr.Number)))
		titles = append(titles, shellEscape(pr.Title))
		authors = append(authors, shellEscape(pr.Author))
	}

	// Replace plural placeholders first
	result := command
	result = strings.ReplaceAll(result, "{urls}", strings.Join(urls, " "))
	result = strings.ReplaceAll(result, "{numbers}", strings.Join(numbers, " "))
	result = strings.ReplaceAll(result, "{titles}", strings.Join(titles, " "))
	result = strings.ReplaceAll(result, "{authors}", strings.Join(authors, " "))

	// Then apply singular placeholders using first PR
	result = SubstitutePlaceholders(result, prs[0])

	return result
}

// shellEscape wraps a string in single quotes and escapes embedded single quotes.
// This prevents shell injection attacks.
func shellEscape(s string) string {
	// Replace single quotes with the sequence: '\''
	// This ends the quoted string, adds an escaped quote, and starts a new quoted string
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
