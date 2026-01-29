package lazypr

import (
	"testing"
)

func TestFilterOptions_GraphQLStates(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{
			name:  "open state",
			state: "open",
			want:  "[OPEN]",
		},
		{
			name:  "closed state",
			state: "closed",
			want:  "[CLOSED, MERGED]",
		},
		{
			name:  "all state",
			state: "all",
			want:  "[OPEN, CLOSED, MERGED]",
		},
		{
			name:  "empty defaults to open",
			state: "",
			want:  "[OPEN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FilterOptions{State: tt.state}
			got := f.GraphQLStates()
			if got != tt.want {
				t.Errorf("GraphQLStates() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterOptions_HasFilters(t *testing.T) {
	tests := []struct {
		name   string
		filter FilterOptions
		want   bool
	}{
		{
			name:   "empty filter",
			filter: FilterOptions{},
			want:   false,
		},
		{
			name:   "only state is not a filter",
			filter: FilterOptions{State: "open"},
			want:   false,
		},
		{
			name:   "with label",
			filter: FilterOptions{Labels: []string{"bug"}},
			want:   true,
		},
		{
			name:   "with author",
			filter: FilterOptions{Author: "octocat"},
			want:   true,
		},
		{
			name:   "with milestone",
			filter: FilterOptions{Milestone: "v1.0"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.HasFilters()
			if got != tt.want {
				t.Errorf("HasFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterOptions_MatchesPR(t *testing.T) {
	pr := PRDetail{
		Author: "octocat",
		Labels: []string{"bug", "help-wanted"},
	}

	tests := []struct {
		name   string
		filter FilterOptions
		want   bool
	}{
		{
			name:   "empty filter matches all",
			filter: FilterOptions{},
			want:   true,
		},
		{
			name:   "matching author",
			filter: FilterOptions{Author: "octocat"},
			want:   true,
		},
		{
			name:   "non-matching author",
			filter: FilterOptions{Author: "other"},
			want:   false,
		},
		{
			name:   "matching single label",
			filter: FilterOptions{Labels: []string{"bug"}},
			want:   true,
		},
		{
			name:   "matching multiple labels",
			filter: FilterOptions{Labels: []string{"bug", "help-wanted"}},
			want:   true,
		},
		{
			name:   "non-matching label",
			filter: FilterOptions{Labels: []string{"enhancement"}},
			want:   false,
		},
		{
			name:   "partial label match fails",
			filter: FilterOptions{Labels: []string{"bug", "missing"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.MatchesPR(pr)
			if got != tt.want {
				t.Errorf("MatchesPR() = %v, want %v", got, tt.want)
			}
		})
	}
}
