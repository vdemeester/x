package lazypr

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// SSH format: git@github.com:owner/repo.git
	sshPattern = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)

	// HTTPS format: https://github.com/owner/repo.git
	httpsPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)

	// ErrNoGitHubRemote indicates no GitHub remote was found.
	ErrNoGitHubRemote = errors.New("no GitHub remote found")

	// ErrNotGitRepo indicates we're not in a git repository.
	ErrNotGitRepo = errors.New("not in a git repository")
)

// ParseGitRemoteURL parses a git remote URL and extracts the GitHub owner/repo.
// Returns the RepoRef and true if it's a valid GitHub URL, false otherwise.
func ParseGitRemoteURL(url string) (RepoRef, bool) {
	url = strings.TrimSpace(url)

	// Try SSH format
	if matches := sshPattern.FindStringSubmatch(url); matches != nil {
		return RepoRef{Owner: matches[1], Repo: matches[2]}, true
	}

	// Try HTTPS format
	if matches := httpsPattern.FindStringSubmatch(url); matches != nil {
		return RepoRef{Owner: matches[1], Repo: matches[2]}, true
	}

	return RepoRef{}, false
}

// ParseGitRemoteOutput parses the output of `git remote -v` and returns
// the best GitHub remote. Prefers "origin" over other remotes.
func ParseGitRemoteOutput(output string) (RepoRef, error) {
	lines := strings.Split(output, "\n")

	var originRef *RepoRef
	var firstGitHubRef *RepoRef

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse: remotename	url (fetch/push)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		remoteName := parts[0]
		remoteURL := parts[1]

		ref, ok := ParseGitRemoteURL(remoteURL)
		if !ok {
			continue
		}

		// Track first GitHub remote as fallback
		if firstGitHubRef == nil {
			firstGitHubRef = &ref
		}

		// Prefer origin
		if remoteName == "origin" {
			originRef = &ref
		}
	}

	// Return origin if found, otherwise first GitHub remote
	if originRef != nil {
		return *originRef, nil
	}
	if firstGitHubRef != nil {
		return *firstGitHubRef, nil
	}

	return RepoRef{}, ErrNoGitHubRemote
}

// DetectGitHubRemote detects the GitHub remote from the current directory.
// It runs `git remote -v` and parses the output.
func DetectGitHubRemote() (RepoRef, error) {
	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return RepoRef{}, ErrNotGitRepo
	}

	// Get remotes
	cmd = exec.Command("git", "remote", "-v")
	output, err := cmd.Output()
	if err != nil {
		return RepoRef{}, ErrNotGitRepo
	}

	return ParseGitRemoteOutput(string(output))
}
