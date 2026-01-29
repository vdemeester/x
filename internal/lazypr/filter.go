package lazypr

import "strings"

// FilterOptions defines filters for PR queries.
type FilterOptions struct {
	Labels    []string // Filter by labels (all must match)
	Milestone string   // Filter by milestone title
	Author    string   // Filter by author username
	State     string   // "open", "closed", or "all" (default: "open")
}

// GraphQLStates returns the GraphQL states array for the filter.
func (f FilterOptions) GraphQLStates() string {
	switch strings.ToLower(f.State) {
	case "closed":
		return "[CLOSED, MERGED]"
	case "all":
		return "[OPEN, CLOSED, MERGED]"
	default: // "open" or empty
		return "[OPEN]"
	}
}

// HasFilters returns true if any filters (other than state) are set.
// This determines if client-side filtering is needed.
func (f FilterOptions) HasFilters() bool {
	return len(f.Labels) > 0 || f.Author != "" || f.Milestone != ""
}

// MatchesPR returns true if the PR matches all filter criteria.
func (f FilterOptions) MatchesPR(pr PRDetail) bool {
	// Check author
	if f.Author != "" && pr.Author != f.Author {
		return false
	}

	// Check labels (all must be present)
	if len(f.Labels) > 0 {
		prLabels := make(map[string]bool)
		for _, l := range pr.Labels {
			prLabels[l] = true
		}
		for _, label := range f.Labels {
			if !prLabels[label] {
				return false
			}
		}
	}

	// Milestone filtering would require additional PR fields
	// For now, we'll handle it in the GraphQL query when possible

	return true
}
