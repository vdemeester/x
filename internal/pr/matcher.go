package pr

import (
	"regexp"
	"strings"
	"sync"

	"go.sbr.pm/x/internal/deps"
)

// Matcher matches PRs against dependencies
type Matcher struct {
	deps *deps.Dependencies

	// Compiled regex patterns for package path matching
	packagePatterns []*regexp.Regexp
	modulePattern   *regexp.Regexp

	// Cache for compiled word boundary regexes
	regexCache sync.Map // map[string]*regexp.Regexp
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
			// First try exact match
			if m.deps.HasModulePath(file.Path) {
				result.addMatch(Match{
					Type:       "module",
					Dependency: file.Path,
					FilePath:   file.Path,
					Confidence: "high",
				})
			} else {
				// Try fuzzy match: check if file path contains any of our service names
				// e.g., "nixos/modules/services/web-servers/nginx.nix" matches "nginx"
				for _, mod := range m.deps.Modules {
					// Extract service name from our synthetic path
					// e.g., "nixos/modules/services/docker" -> "docker"
					serviceName := extractServiceName(mod.Path)
					if serviceName != "" && strings.Contains(file.Path, serviceName) {
						if !result.hasMatch(mod.Path) {
							result.addMatch(Match{
								Type:       "module",
								Dependency: serviceName,
								FilePath:   file.Path,
								Confidence: "high",
							})
						}
						break
					}
				}
			}
		}
	}

	// Phase 2: Title matching (medium confidence)
	titleLower := strings.ToLower(pr.Title)
	for _, pkg := range m.deps.Packages {
		pkgLower := strings.ToLower(pkg.Name)

		// Use word boundary matching to avoid false positives
		// For short names (< 3 chars), require exact word match
		// For longer names, use word boundaries
		matched := false
		if len(pkg.Name) < 3 {
			// Short names: require exact word match (surrounded by non-alphanumeric)
			matched = m.isExactWordMatch(titleLower, pkgLower)
		} else {
			// Longer names: use word boundary regex
			matched = m.matchesWithWordBoundary(titleLower, pkgLower)
		}

		if matched {
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

// extractServiceName extracts service name from a module path
// e.g., "nixos/modules/services/docker" -> "docker"
func extractServiceName(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// isExactWordMatch checks if pkg appears as a complete word in text
// Used for short package names to avoid false positives (e.g., "oc" in "ocaml")
func (m *Matcher) isExactWordMatch(text, pkg string) bool {
	// Check if pkg appears as a standalone word surrounded by non-alphanumeric chars
	// or at the start/end of the text
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	})

	for _, word := range words {
		if word == pkg {
			return true
		}
	}
	return false
}

// matchesWithWordBoundary checks if pkg matches with word boundaries using regex
func (m *Matcher) matchesWithWordBoundary(text, pkg string) bool {
	// For packages without hyphens, check if text contains pkg followed by hyphen
	// This prevents "openssh" from matching "openssh-client"
	// (Go regex doesn't support negative lookahead)
	if !strings.Contains(pkg, "-") {
		pkgWithHyphen := strings.ToLower(pkg) + "-"
		if strings.Contains(strings.ToLower(text), pkgWithHyphen) {
			return false
		}
	}

	// Check cache first
	cacheKey := pkg
	if cached, ok := m.regexCache.Load(cacheKey); ok {
		return cached.(*regexp.Regexp).MatchString(text)
	}

	// Build pattern with case-insensitive flag and word boundaries
	patternStr := `(?i)\b` + regexp.QuoteMeta(pkg) + `\b`
	pattern := regexp.MustCompile(patternStr)

	// Cache the compiled pattern
	m.regexCache.Store(cacheKey, pattern)

	return pattern.MatchString(text)
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
