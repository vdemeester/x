package pr

import (
	"testing"

	"go.sbr.pm/x/internal/deps"
)

func TestMatcher_isExactWordMatch(t *testing.T) {
	tests := []struct {
		name string
		text string
		pkg  string
		want bool
	}{
		{
			name: "exact match at start",
			text: "oc update",
			pkg:  "oc",
			want: true,
		},
		{
			name: "exact match at end",
			text: "update oc",
			pkg:  "oc",
			want: true,
		},
		{
			name: "exact match in middle",
			text: "update oc now",
			pkg:  "oc",
			want: true,
		},
		{
			name: "substring should not match",
			text: "ocaml update",
			pkg:  "oc",
			want: false,
		},
		{
			name: "substring in middle should not match",
			text: "update ocaml now",
			pkg:  "oc",
			want: false,
		},
		{
			name: "hyphenated package should match",
			text: "my-pkg update",
			pkg:  "my-pkg",
			want: true,
		},
		{
			name: "with colon separator",
			text: "oc: update to 1.2.3",
			pkg:  "oc",
			want: true,
		},
		{
			name: "empty text",
			text: "",
			pkg:  "oc",
			want: false,
		},
		{
			name: "empty package",
			text: "some text",
			pkg:  "",
			want: false,
		},
	}

	m := NewMatcher(&deps.Dependencies{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.isExactWordMatch(tt.text, tt.pkg)
			if got != tt.want {
				t.Errorf("isExactWordMatch(%q, %q) = %v, want %v", tt.text, tt.pkg, got, tt.want)
			}
		})
	}
}

func TestMatcher_matchesWithWordBoundary(t *testing.T) {
	tests := []struct {
		name string
		text string
		pkg  string
		want bool
	}{
		{
			name: "exact match",
			text: "git update",
			pkg:  "git",
			want: true,
		},
		{
			name: "substring should not match - age in kdePackages",
			text: "kdePa ckages update",
			pkg:  "age",
			want: false,
		},
		{
			name: "substring should not match - age in manage",
			text: "manage update",
			pkg:  "age",
			want: false,
		},
		{
			name: "word boundary at start",
			text: "git: update to 2.44.0",
			pkg:  "git",
			want: true,
		},
		{
			name: "word boundary at end",
			text: "update git",
			pkg:  "git",
			want: true,
		},
		{
			name: "hyphenated package",
			text: "openssh-client update",
			pkg:  "openssh-client",
			want: true,
		},
		{
			name: "should not match partial hyphenated",
			text: "openssh-client update",
			pkg:  "openssh",
			want: false,
		},
		{
			name: "case insensitive",
			text: "GIT update",
			pkg:  "git",
			want: true,
		},
	}

	m := NewMatcher(&deps.Dependencies{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.matchesWithWordBoundary(tt.text, tt.pkg)
			if got != tt.want {
				t.Errorf("matchesWithWordBoundary(%q, %q) = %v, want %v", tt.text, tt.pkg, got, tt.want)
			}
		})
	}
}

// Benchmark the regex compilation issue
func BenchmarkMatchesWithWordBoundary_Uncached(b *testing.B) {
	m := NewMatcher(&deps.Dependencies{})
	text := "git: update to version 2.44.0"
	pkg := "git"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.matchesWithWordBoundary(text, pkg)
	}
}

func BenchmarkMatchPR_WithManyPackages(b *testing.B) {
	// Simulate realistic scenario: 392 packages
	packages := make([]deps.Package, 392)
	for i := 0; i < 392; i++ {
		packages[i] = deps.Package{Name: "package" + string(rune(i))}
	}

	dependencies := &deps.Dependencies{
		Packages: packages,
	}

	m := NewMatcher(dependencies)

	pr := PullRequest{
		Number: 1,
		Title:  "package100: update to 1.2.3",
		Files:  []File{{Path: "pkgs/by-name/pa/package100/package.nix"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.matchPR(pr)
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		want       string
	}{
		{
			name:       "simple service",
			modulePath: "nixos/modules/services/docker",
			want:       "docker",
		},
		{
			name:       "nested service",
			modulePath: "nixos/modules/services/networking/ssh",
			want:       "ssh",
		},
		{
			name:       "single component",
			modulePath: "nginx",
			want:       "nginx",
		},
		{
			name:       "empty path",
			modulePath: "",
			want:       "",
		},
		{
			name:       "trailing slash",
			modulePath: "nixos/modules/services/docker/",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceName(tt.modulePath)
			if got != tt.want {
				t.Errorf("extractServiceName(%q) = %q, want %q", tt.modulePath, got, tt.want)
			}
		})
	}
}
