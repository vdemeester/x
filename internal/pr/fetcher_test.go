package pr

import (
	"testing"
)

func TestFilterByBaseBranch(t *testing.T) {
	tests := []struct {
		name       string
		prs        []PullRequest
		baseBranch string
		want       int // expected number of PRs after filtering
	}{
		{
			name: "filter to master only",
			prs: []PullRequest{
				{Number: 1, Title: "PR to master", BaseRef: "master"},
				{Number: 2, Title: "PR to staging", BaseRef: "staging"},
				{Number: 3, Title: "Another master PR", BaseRef: "master"},
			},
			baseBranch: "master",
			want:       2,
		},
		{
			name: "filter to staging only",
			prs: []PullRequest{
				{Number: 1, Title: "PR to master", BaseRef: "master"},
				{Number: 2, Title: "PR to staging", BaseRef: "staging"},
				{Number: 3, Title: "Another staging PR", BaseRef: "staging"},
			},
			baseBranch: "staging",
			want:       2,
		},
		{
			name: "no matches",
			prs: []PullRequest{
				{Number: 1, Title: "PR to master", BaseRef: "master"},
				{Number: 2, Title: "PR to staging", BaseRef: "staging"},
			},
			baseBranch: "release",
			want:       0,
		},
		{
			name: "all match same branch",
			prs: []PullRequest{
				{Number: 1, Title: "PR 1", BaseRef: "master"},
				{Number: 2, Title: "PR 2", BaseRef: "master"},
				{Number: 3, Title: "PR 3", BaseRef: "master"},
			},
			baseBranch: "master",
			want:       3,
		},
		{
			name:       "empty PR list",
			prs:        []PullRequest{},
			baseBranch: "master",
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterByBaseBranch(tt.prs, tt.baseBranch)
			if len(filtered) != tt.want {
				t.Errorf("FilterByBaseBranch() returned %d PRs, want %d", len(filtered), tt.want)
			}

			// Verify all returned PRs match the base branch
			for _, pr := range filtered {
				if pr.BaseRef != tt.baseBranch {
					t.Errorf("PR #%d has BaseRef=%q, want %q", pr.Number, pr.BaseRef, tt.baseBranch)
				}
			}
		})
	}
}

func TestPullRequest_HasBaseBranch(t *testing.T) {
	tests := []struct {
		name       string
		pr         PullRequest
		baseBranch string
		want       bool
	}{
		{
			name:       "matches master",
			pr:         PullRequest{Number: 1, BaseRef: "master"},
			baseBranch: "master",
			want:       true,
		},
		{
			name:       "does not match",
			pr:         PullRequest{Number: 1, BaseRef: "staging"},
			baseBranch: "master",
			want:       false,
		},
		{
			name:       "empty base ref",
			pr:         PullRequest{Number: 1, BaseRef: ""},
			baseBranch: "master",
			want:       false,
		},
		{
			name:       "case sensitive",
			pr:         PullRequest{Number: 1, BaseRef: "Master"},
			baseBranch: "master",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.HasBaseBranch(tt.baseBranch)
			if got != tt.want {
				t.Errorf("HasBaseBranch(%q) = %v, want %v", tt.baseBranch, got, tt.want)
			}
		})
	}
}
