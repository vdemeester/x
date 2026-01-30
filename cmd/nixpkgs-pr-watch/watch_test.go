package main

import (
	"bytes"
	"testing"
	"time"

	"go.sbr.pm/x/internal/pr"
)

func TestOutputURLs(t *testing.T) {
	// Create test match results with known URLs
	results := []pr.MatchResult{
		{
			PR: pr.PullRequest{
				Number: 123,
				Title:  "test: update foo package",
				URL:    "https://github.com/NixOS/nixpkgs/pull/123",
			},
		},
		{
			PR: pr.PullRequest{
				Number: 456,
				Title:  "fix: bar security issue",
				URL:    "https://github.com/NixOS/nixpkgs/pull/456",
			},
		},
		{
			PR: pr.PullRequest{
				Number: 789,
				Title:  "feat: add baz package",
				URL:    "https://github.com/NixOS/nixpkgs/pull/789",
			},
		},
	}

	var buf bytes.Buffer
	err := outputURLs(&buf, results)
	if err != nil {
		t.Fatalf("outputURLs() returned error: %v", err)
	}

	got := buf.String()
	want := `https://github.com/NixOS/nixpkgs/pull/123
https://github.com/NixOS/nixpkgs/pull/456
https://github.com/NixOS/nixpkgs/pull/789
`

	if got != want {
		t.Errorf("outputURLs() =\n%q\nwant:\n%q", got, want)
	}
}

func TestOutputURLs_Empty(t *testing.T) {
	var results []pr.MatchResult
	var buf bytes.Buffer

	err := outputURLs(&buf, results)
	if err != nil {
		t.Fatalf("outputURLs() returned error: %v", err)
	}

	got := buf.String()
	if got != "" {
		t.Errorf("outputURLs() with empty results = %q, want empty string", got)
	}
}

func TestOutputURLs_SingleResult(t *testing.T) {
	results := []pr.MatchResult{
		{
			PR: pr.PullRequest{
				Number:    42,
				Title:     "single PR",
				URL:       "https://github.com/NixOS/nixpkgs/pull/42",
				CreatedAt: time.Now(),
			},
		},
	}

	var buf bytes.Buffer
	err := outputURLs(&buf, results)
	if err != nil {
		t.Fatalf("outputURLs() returned error: %v", err)
	}

	got := buf.String()
	want := "https://github.com/NixOS/nixpkgs/pull/42\n"

	if got != want {
		t.Errorf("outputURLs() = %q, want %q", got, want)
	}
}
