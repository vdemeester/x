package lazypr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Match GitHub PR URLs: https://github.com/owner/repo/pull/123
	urlPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

	// Match short format: owner/repo#123
	shortPattern = regexp.MustCompile(`^([^/]+)/([^#]+)#(\d+)$`)
)

// ParsePRRef parses a PR reference from various formats.
// Supported formats:
//   - https://github.com/owner/repo/pull/123
//   - owner/repo#123
func ParsePRRef(input string) (PRRef, error) {
	input = strings.TrimSpace(input)

	// Try URL format first
	if matches := urlPattern.FindStringSubmatch(input); matches != nil {
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return PRRef{}, fmt.Errorf("invalid PR number: %s", matches[3])
		}
		return PRRef{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: num,
		}, nil
	}

	// Try short format: owner/repo#123
	if matches := shortPattern.FindStringSubmatch(input); matches != nil {
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return PRRef{}, fmt.Errorf("invalid PR number: %s", matches[3])
		}
		return PRRef{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: num,
		}, nil
	}

	return PRRef{}, fmt.Errorf("invalid PR reference: %s (use owner/repo#123 or GitHub URL)", input)
}

// ParsePRRefs parses multiple PR references.
func ParsePRRefs(inputs []string) ([]PRRef, error) {
	refs := make([]PRRef, 0, len(inputs))
	for _, input := range inputs {
		ref, err := ParsePRRef(input)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// String returns the string representation of a PRRef.
func (r PRRef) String() string {
	return fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number)
}

// URL returns the GitHub URL for this PR.
func (r PRRef) URL() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", r.Owner, r.Repo, r.Number)
}
