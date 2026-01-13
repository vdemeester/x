package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config represents the flake configuration
type Config struct {
	flakePath string
}

// New creates a new Config instance
func New(flakePath string) (*Config, error) {
	// Verify flake.nix exists
	flakeFile := filepath.Join(flakePath, "flake.nix")
	if _, err := os.Stat(flakeFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("flake.nix not found at %s", flakePath)
	}

	return &Config{
		flakePath: flakePath,
	}, nil
}

// FlakePath returns the path to the flake directory
func (c *Config) FlakePath() string {
	return c.flakePath
}

// AllHosts returns all NixOS hosts defined in the flake
func (c *Config) AllHosts() ([]string, error) {
	// Use nix flake show to list all nixosConfigurations
	cmd := exec.Command("nix", "flake", "show", "--json", c.flakePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run nix flake show: %w", err)
	}

	var flakeInfo map[string]interface{}
	if err := json.Unmarshal(output, &flakeInfo); err != nil {
		return nil, fmt.Errorf("failed to parse flake info: %w", err)
	}

	// Extract nixosConfigurations
	nixosConfigs, ok := flakeInfo["nixosConfigurations"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no nixosConfigurations found in flake")
	}

	hosts := make([]string, 0, len(nixosConfigs))
	for host := range nixosConfigs {
		hosts = append(hosts, host)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in flake")
	}

	return hosts, nil
}

// CurrentHost returns the current hostname
func (c *Config) CurrentHost() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	// Strip domain if present
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}

	// Verify this host exists in the flake
	hosts, err := c.AllHosts()
	if err != nil {
		return "", err
	}

	for _, h := range hosts {
		if h == hostname {
			return hostname, nil
		}
	}

	return "", fmt.Errorf("current host %q not found in flake (available: %v)", hostname, hosts)
}
