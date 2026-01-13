package pr

import (
	"regexp"
	"strings"

	"go.sbr.pm/x/internal/deps"
)

// Matcher matches PRs against dependencies
type Matcher struct {
	deps *deps.Dependencies

	// Compiled regex patterns for package path matching
	packagePatterns []*regexp.Regexp
	modulePattern   *regexp.Regexp
}

// NewMatcher creates a new PR matcher
func NewMatcher(dependencies *deps.Dependencies) *Matcher {
	// Compile package path patterns
	packagePatterns := []*regexp.Regexp{
		// pkgs/by-name/gi/git/package.nix → "git"
		regexp.MustCompile(`pkgs/by-name/[^/]+/([^/]+)/`),
		// pkgs/development/tools/git/default.nix → "git"
		regexp.MustCompile(`pkgs/.*/([^/]+)/default\.nix$`),
		// pkgs/development/tools/git/package.nix → "git"
		regexp.MustCompile(`pkgs/.*/([^/]+)/package\.nix$`),
	}

	// Module path pattern
	modulePattern := regexp.MustCompile(`^(nixos|home-manager)/modules/`)

	return &Matcher{
		deps:            dependencies,
		packagePatterns: packagePatterns,
		modulePattern:   modulePattern,
	}
}

// MatchAll matches all PRs against dependencies
func (m *Matcher) MatchAll(prs []PullRequest) []MatchResult {
	results := []MatchResult{}

	for _, pr := range prs {
		result := m.matchPR(pr)
		if len(result.Matches) > 0 {
			results = append(results, result)
		}
	}

	return results
}

// matchPR matches a single PR against dependencies
func (m *Matcher) matchPR(pr PullRequest) MatchResult {
	result := MatchResult{
		PR:      pr,
		Score:   0,
		Matches: []Match{},
	}

	// Phase 1: File path matching (highest confidence)
	for _, file := range pr.Files {
		// Check if it's a package file
		pkgName := m.extractPackageName(file.Path)
		if pkgName != "" && m.deps.HasPackage(pkgName) {
			result.addMatch(Match{
				Type:       "package",
				Dependency: pkgName,
				FilePath:   file.Path,
				Confidence: "high",
			})
			continue
		}

		// Check if it's a module file
		if m.modulePattern.MatchString(file.Path) {
			if m.deps.HasModulePath(file.Path) {
				result.addMatch(Match{
					Type:       "module",
					Dependency: file.Path,
					FilePath:   file.Path,
					Confidence: "high",
				})
			}
		}
	}

	// Phase 2: Title matching (medium confidence)
	titleLower := strings.ToLower(pr.Title)
	for _, pkg := range m.deps.Packages {
		pkgLower := strings.ToLower(pkg.Name)
		if strings.Contains(titleLower, pkgLower) {
			// Avoid duplicates from file matching
			if !result.hasMatch(pkg.Name) {
				result.addMatch(Match{
					Type:       "title",
					Dependency: pkg.Name,
					Confidence: "medium",
				})
			}
		}
	}

	return result
}

// extractPackageName extracts package name from a file path
func (m *Matcher) extractPackageName(path string) string {
	for _, pattern := range m.packagePatterns {
		matches := pattern.FindStringSubmatch(path)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// addMatch adds a match to the result and updates the score
func (mr *MatchResult) addMatch(match Match) {
	mr.Matches = append(mr.Matches, match)
	mr.TotalMatches = len(mr.Matches)

	// Update score based on confidence
	switch match.Confidence {
	case "high":
		mr.Score += 100
	case "medium":
		mr.Score += 50
	case "low":
		mr.Score += 25
	}

	// Cap score at 100
	if mr.Score > 100 {
		mr.Score = 100
	}
}

// hasMatch checks if a dependency is already matched
func (mr *MatchResult) hasMatch(dependency string) bool {
	for _, m := range mr.Matches {
		if m.Dependency == dependency {
			return true
		}
	}
	return false
}
