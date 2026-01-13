package pr

import (
	"testing"

	"go.sbr.pm/x/internal/deps"
)

func TestMatcher_extractPackageName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "by-name structure",
			path: "pkgs/by-name/gi/git/package.nix",
			want: "git",
		},
		{
			name: "by-name with default.nix",
			path: "pkgs/by-name/oc/oci-cli/default.nix",
			want: "oci-cli",
		},
		{
			name: "development tools default.nix",
			path: "pkgs/development/tools/git/default.nix",
			want: "git",
		},
		{
			name: "applications package.nix",
			path: "pkgs/applications/version-management/git/package.nix",
			want: "git",
		},
		{
			name: "deep nested path",
			path: "pkgs/development/libraries/openssl/default.nix",
			want: "openssl",
		},
		{
			name: "module path should not match",
			path: "nixos/modules/services/networking/ssh/sshd.nix",
			want: "",
		},
		{
			name: "random nix file should not match",
			path: "flake.nix",
			want: "",
		},
		{
			name: "package directory without file",
			path: "pkgs/by-name/gi/git",
			want: "",
		},
	}

	m := NewMatcher(&deps.Dependencies{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.extractPackageName(tt.path)
			if got != tt.want {
				t.Errorf("extractPackageName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMatcher_matchPR(t *testing.T) {
	dependencies := &deps.Dependencies{
		Packages: []deps.Package{
			{Name: "git"},
			{Name: "openssh"},
			{Name: "nautilus"},
		},
		Modules: []deps.ModulePath{
			{Path: "nixos/modules/services/networking/ssh/sshd.nix", Type: "nixos"},
		},
	}

	tests := []struct {
		name             string
		pr               PullRequest
		wantMatchCount   int
		wantConfidence   string
		wantMatchTypes   []string
		wantDependencies []string
	}{
		{
			name: "high confidence package match",
			pr: PullRequest{
				Number: 1,
				Title:  "git: 2.43.0 -> 2.44.0",
				Files: []File{
					{Path: "pkgs/by-name/gi/git/package.nix"},
				},
			},
			wantMatchCount:   1,
			wantConfidence:   "high",
			wantMatchTypes:   []string{"package"},
			wantDependencies: []string{"git"},
		},
		{
			name: "medium confidence title match",
			pr: PullRequest{
				Number: 2,
				Title:  "openssh: fix CVE-2024-1234",
				Files: []File{
					{Path: "pkgs/tools/networking/other-tool/default.nix"},
				},
			},
			wantMatchCount:   1,
			wantConfidence:   "medium",
			wantMatchTypes:   []string{"title"},
			wantDependencies: []string{"openssh"},
		},
		{
			name: "both package and title match (deduplicated)",
			pr: PullRequest{
				Number: 3,
				Title:  "git: update to latest version",
				Files: []File{
					{Path: "pkgs/by-name/gi/git/package.nix"},
				},
			},
			wantMatchCount:   1,
			wantConfidence:   "high",
			wantMatchTypes:   []string{"package"},
			wantDependencies: []string{"git"},
		},
		{
			name: "multiple file matches",
			pr: PullRequest{
				Number: 4,
				Title:  "Update multiple packages",
				Files: []File{
					{Path: "pkgs/by-name/gi/git/package.nix"},
					{Path: "pkgs/by-name/op/openssh/package.nix"},
				},
			},
			wantMatchCount:   2,
			wantConfidence:   "high",
			wantMatchTypes:   []string{"package", "package"},
			wantDependencies: []string{"git", "openssh"},
		},
		{
			name: "module path match",
			pr: PullRequest{
				Number: 5,
				Title:  "sshd: improve security",
				Files: []File{
					{Path: "nixos/modules/services/networking/ssh/sshd.nix"},
				},
			},
			wantMatchCount:   1,
			wantConfidence:   "high",
			wantMatchTypes:   []string{"module"},
			wantDependencies: []string{"nixos/modules/services/networking/ssh/sshd.nix"},
		},
		{
			name: "no match",
			pr: PullRequest{
				Number: 6,
				Title:  "random-package: update",
				Files: []File{
					{Path: "pkgs/by-name/ra/random-package/package.nix"},
				},
			},
			wantMatchCount: 0,
		},
		{
			name: "GNOME updates matching nautilus",
			pr: PullRequest{
				Number: 7,
				Title:  "GNOME updates 2026-01-13",
				Files: []File{
					{Path: "pkgs/by-name/eo/eog/package.nix"},
					{Path: "pkgs/by-name/na/nautilus/package.nix"},
				},
			},
			wantMatchCount:   1,
			wantConfidence:   "high",
			wantMatchTypes:   []string{"package"},
			wantDependencies: []string{"nautilus"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatcher(dependencies)
			result := m.matchPR(tt.pr)

			if len(result.Matches) != tt.wantMatchCount {
				t.Errorf("matchPR() got %d matches, want %d", len(result.Matches), tt.wantMatchCount)
			}

			if tt.wantMatchCount == 0 {
				return
			}

			if result.HighestConfidence() != tt.wantConfidence {
				t.Errorf("HighestConfidence() = %q, want %q", result.HighestConfidence(), tt.wantConfidence)
			}

			// Verify match types
			for i, match := range result.Matches {
				if i >= len(tt.wantMatchTypes) {
					break
				}
				if match.Type != tt.wantMatchTypes[i] {
					t.Errorf("Match[%d].Type = %q, want %q", i, match.Type, tt.wantMatchTypes[i])
				}
			}

			// Verify dependencies
			for i, match := range result.Matches {
				if i >= len(tt.wantDependencies) {
					break
				}
				if match.Dependency != tt.wantDependencies[i] {
					t.Errorf("Match[%d].Dependency = %q, want %q", i, match.Dependency, tt.wantDependencies[i])
				}
			}
		})
	}
}

func TestMatchResult_HighestConfidence(t *testing.T) {
	tests := []struct {
		name    string
		matches []Match
		want    string
	}{
		{
			name: "single high confidence",
			matches: []Match{
				{Confidence: "high"},
			},
			want: "high",
		},
		{
			name: "single medium confidence",
			matches: []Match{
				{Confidence: "medium"},
			},
			want: "medium",
		},
		{
			name: "single low confidence",
			matches: []Match{
				{Confidence: "low"},
			},
			want: "low",
		},
		{
			name: "high among mixed",
			matches: []Match{
				{Confidence: "medium"},
				{Confidence: "high"},
				{Confidence: "low"},
			},
			want: "high",
		},
		{
			name: "medium among low",
			matches: []Match{
				{Confidence: "low"},
				{Confidence: "medium"},
			},
			want: "medium",
		},
		{
			name:    "no matches",
			matches: []Match{},
			want:    "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := &MatchResult{Matches: tt.matches}
			got := mr.HighestConfidence()
			if got != tt.want {
				t.Errorf("HighestConfidence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchResult_addMatch(t *testing.T) {
	tests := []struct {
		name      string
		existing  []Match
		newMatch  Match
		wantCount int
		wantScore int
	}{
		{
			name:     "add high confidence match",
			existing: []Match{},
			newMatch: Match{
				Type:       "package",
				Dependency: "git",
				Confidence: "high",
			},
			wantCount: 1,
			wantScore: 100,
		},
		{
			name:     "add medium confidence match",
			existing: []Match{},
			newMatch: Match{
				Type:       "title",
				Dependency: "openssh",
				Confidence: "medium",
			},
			wantCount: 1,
			wantScore: 50,
		},
		{
			name:     "add low confidence match",
			existing: []Match{},
			newMatch: Match{
				Type:       "heuristic",
				Dependency: "curl",
				Confidence: "low",
			},
			wantCount: 1,
			wantScore: 25,
		},
		{
			name: "multiple matches cap at 100",
			existing: []Match{
				{Confidence: "high"},
				{Confidence: "medium"},
			},
			newMatch: Match{
				Type:       "package",
				Dependency: "git",
				Confidence: "high",
			},
			wantCount: 3,
			wantScore: 100, // Capped at 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := &MatchResult{
				Matches: tt.existing,
			}

			// Calculate initial score
			for _, m := range tt.existing {
				switch m.Confidence {
				case "high":
					mr.Score += 100
				case "medium":
					mr.Score += 50
				case "low":
					mr.Score += 25
				}
			}
			if mr.Score > 100 {
				mr.Score = 100
			}

			mr.addMatch(tt.newMatch)

			if len(mr.Matches) != tt.wantCount {
				t.Errorf("addMatch() got %d matches, want %d", len(mr.Matches), tt.wantCount)
			}

			if mr.Score != tt.wantScore {
				t.Errorf("addMatch() Score = %d, want %d", mr.Score, tt.wantScore)
			}

			if mr.TotalMatches != tt.wantCount {
				t.Errorf("addMatch() TotalMatches = %d, want %d", mr.TotalMatches, tt.wantCount)
			}
		})
	}
}

func TestMatchResult_hasMatch(t *testing.T) {
	mr := &MatchResult{
		Matches: []Match{
			{Dependency: "git"},
			{Dependency: "openssh"},
		},
	}

	tests := []struct {
		name       string
		dependency string
		want       bool
	}{
		{
			name:       "existing dependency",
			dependency: "git",
			want:       true,
		},
		{
			name:       "another existing dependency",
			dependency: "openssh",
			want:       true,
		},
		{
			name:       "non-existing dependency",
			dependency: "curl",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mr.hasMatch(tt.dependency)
			if got != tt.want {
				t.Errorf("hasMatch(%q) = %v, want %v", tt.dependency, got, tt.want)
			}
		})
	}
}

func TestMatcher_MatchAll(t *testing.T) {
	dependencies := &deps.Dependencies{
		Packages: []deps.Package{
			{Name: "git"},
			{Name: "openssh"},
		},
	}

	prs := []PullRequest{
		{
			Number: 1,
			Title:  "git: update",
			Files: []File{
				{Path: "pkgs/by-name/gi/git/package.nix"},
			},
		},
		{
			Number: 2,
			Title:  "random: update",
			Files: []File{
				{Path: "pkgs/by-name/ra/random/package.nix"},
			},
		},
		{
			Number: 3,
			Title:  "openssh: security fix",
			Files: []File{
				{Path: "pkgs/tools/networking/openssh/default.nix"},
			},
		},
	}

	m := NewMatcher(dependencies)
	results := m.MatchAll(prs)

	// Should match git and openssh, but not random
	if len(results) != 2 {
		t.Errorf("MatchAll() got %d results, want 2", len(results))
	}

	// Verify we got the right PRs
	wantNumbers := map[int]bool{1: true, 3: true}
	for _, result := range results {
		if !wantNumbers[result.PR.Number] {
			t.Errorf("MatchAll() included unexpected PR #%d", result.PR.Number)
		}
	}
}
