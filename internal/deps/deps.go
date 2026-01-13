package deps

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Package represents a package dependency
type Package struct {
	Name      string `json:"name"`
	Attribute string `json:"attribute,omitempty"`
}

// ModulePath represents a NixOS or home-manager module
type ModulePath struct {
	Path string `json:"path"`
	Type string `json:"type"` // "nixos" or "home-manager"
}

// Dependencies contains all extracted dependencies
type Dependencies struct {
	Packages []Package    `json:"packages"`
	Modules  []ModulePath `json:"modules"`
	Services []string     `json:"services"`
}

// Extractor extracts dependencies from a NixOS configuration
type Extractor struct {
	flakePath string
	hostname  string
}

// NewExtractor creates a new dependency extractor
func NewExtractor(flakePath, hostname string) *Extractor {
	return &Extractor{
		flakePath: flakePath,
		hostname:  hostname,
	}
}

// Extract extracts all dependencies from the configuration
func (e *Extractor) Extract() (Dependencies, error) {
	deps := Dependencies{
		Packages: []Package{},
		Modules:  []ModulePath{},
		Services: []string{},
	}

	// Extract system packages
	systemPkgs, err := e.extractSystemPackages()
	if err != nil {
		return deps, fmt.Errorf("failed to extract system packages: %w", err)
	}
	deps.Packages = append(deps.Packages, systemPkgs...)

	// Extract home-manager packages (if available)
	homePkgs, err := e.extractHomePackages()
	if err != nil {
		// Home-manager might not be configured, that's ok
		// Just log and continue
	} else {
		deps.Packages = append(deps.Packages, homePkgs...)
	}

	// Deduplicate packages
	deps.Packages = deduplicatePackages(deps.Packages)

	return deps, nil
}

// extractSystemPackages extracts packages from environment.systemPackages
func (e *Extractor) extractSystemPackages() ([]Package, error) {
	flakeRef := fmt.Sprintf("%s#nixosConfigurations.%s.config.environment.systemPackages", e.flakePath, e.hostname)

	cmd := exec.Command("nix", "eval", flakeRef,
		"--apply", `pkgs: map (p: p.pname or p.name or "unknown") pkgs`,
		"--json")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nix eval failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("failed to parse package names: %w", err)
	}

	packages := make([]Package, 0, len(names))
	for _, name := range names {
		if name != "" && name != "unknown" {
			packages = append(packages, Package{Name: name})
		}
	}

	return packages, nil
}

// extractHomePackages extracts packages from home-manager configuration
func (e *Extractor) extractHomePackages() ([]Package, error) {
	// Try common home-manager paths
	usernames := []string{"vincent", "vdemeest"}

	for _, username := range usernames {
		flakeRef := fmt.Sprintf("%s#nixosConfigurations.%s.config.home-manager.users.%s.home.packages",
			e.flakePath, e.hostname, username)

		cmd := exec.Command("nix", "eval", flakeRef,
			"--apply", `pkgs: map (p: p.pname or p.name or "unknown") pkgs`,
			"--json")

		output, err := cmd.Output()
		if err != nil {
			// Try next username
			continue
		}

		var names []string
		if err := json.Unmarshal(output, &names); err != nil {
			continue
		}

		packages := make([]Package, 0, len(names))
		for _, name := range names {
			if name != "" && name != "unknown" {
				packages = append(packages, Package{Name: name})
			}
		}

		return packages, nil
	}

	return nil, fmt.Errorf("no home-manager packages found")
}

// deduplicatePackages removes duplicate packages
func deduplicatePackages(packages []Package) []Package {
	seen := make(map[string]bool)
	result := []Package{}

	for _, pkg := range packages {
		key := pkg.Name
		if !seen[key] {
			seen[key] = true
			result = append(result, pkg)
		}
	}

	return result
}

// Merge merges dependencies from multiple hosts
func Merge(hostDeps map[string]*Dependencies) *Dependencies {
	merged := &Dependencies{
		Packages: []Package{},
		Modules:  []ModulePath{},
		Services: []string{},
	}

	pkgSeen := make(map[string]bool)
	modSeen := make(map[string]bool)
	svcSeen := make(map[string]bool)

	for _, deps := range hostDeps {
		for _, pkg := range deps.Packages {
			if !pkgSeen[pkg.Name] {
				pkgSeen[pkg.Name] = true
				merged.Packages = append(merged.Packages, pkg)
			}
		}

		for _, mod := range deps.Modules {
			key := fmt.Sprintf("%s:%s", mod.Type, mod.Path)
			if !modSeen[key] {
				modSeen[key] = true
				merged.Modules = append(merged.Modules, mod)
			}
		}

		for _, svc := range deps.Services {
			if !svcSeen[svc] {
				svcSeen[svc] = true
				merged.Services = append(merged.Services, svc)
			}
		}
	}

	return merged
}

// HasPackage checks if dependencies contain a package with the given name
func (d *Dependencies) HasPackage(name string) bool {
	name = strings.ToLower(name)
	for _, pkg := range d.Packages {
		if strings.ToLower(pkg.Name) == name {
			return true
		}
	}
	return false
}

// HasModulePath checks if dependencies contain a module with the given path
func (d *Dependencies) HasModulePath(path string) bool {
	for _, mod := range d.Modules {
		if mod.Path == path {
			return true
		}
	}
	return false
}
