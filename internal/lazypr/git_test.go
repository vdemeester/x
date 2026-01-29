package lazypr

import (
	"testing"
)

func TestParseGitRemoteURL_SSH(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    RepoRef
		wantOK  bool
	}{
		{
			name:   "standard SSH format",
			url:    "git@github.com:owner/repo.git",
			want:   RepoRef{Owner: "owner", Repo: "repo"},
			wantOK: true,
		},
		{
			name:   "SSH without .git suffix",
			url:    "git@github.com:owner/repo",
			want:   RepoRef{Owner: "owner", Repo: "repo"},
			wantOK: true,
		},
		{
			name:   "SSH with hyphenated names",
			url:    "git@github.com:my-org/my-repo.git",
			want:   RepoRef{Owner: "my-org", Repo: "my-repo"},
			wantOK: true,
		},
		{
			name:   "non-GitHub SSH",
			url:    "git@gitlab.com:owner/repo.git",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGitRemoteURL(tt.url)
			if ok != tt.wantOK {
				t.Errorf("ParseGitRemoteURL(%q) ok = %v, want %v", tt.url, ok, tt.wantOK)
				return
			}
			if ok && got != tt.want {
				t.Errorf("ParseGitRemoteURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseGitRemoteURL_HTTPS(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		want   RepoRef
		wantOK bool
	}{
		{
			name:   "standard HTTPS format",
			url:    "https://github.com/owner/repo.git",
			want:   RepoRef{Owner: "owner", Repo: "repo"},
			wantOK: true,
		},
		{
			name:   "HTTPS without .git suffix",
			url:    "https://github.com/owner/repo",
			want:   RepoRef{Owner: "owner", Repo: "repo"},
			wantOK: true,
		},
		{
			name:   "HTTPS with hyphenated names",
			url:    "https://github.com/my-org/my-repo.git",
			want:   RepoRef{Owner: "my-org", Repo: "my-repo"},
			wantOK: true,
		},
		{
			name:   "HTTP (not HTTPS)",
			url:    "http://github.com/owner/repo.git",
			want:   RepoRef{Owner: "owner", Repo: "repo"},
			wantOK: true,
		},
		{
			name:   "non-GitHub HTTPS",
			url:    "https://gitlab.com/owner/repo.git",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGitRemoteURL(tt.url)
			if ok != tt.wantOK {
				t.Errorf("ParseGitRemoteURL(%q) ok = %v, want %v", tt.url, ok, tt.wantOK)
				return
			}
			if ok && got != tt.want {
				t.Errorf("ParseGitRemoteURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseGitRemoteOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    RepoRef
		wantErr bool
	}{
		{
			name: "origin SSH",
			output: `origin	git@github.com:tektoncd/pipeline.git (fetch)
origin	git@github.com:tektoncd/pipeline.git (push)
`,
			want: RepoRef{Owner: "tektoncd", Repo: "pipeline"},
		},
		{
			name: "origin HTTPS",
			output: `origin	https://github.com/tektoncd/pipeline.git (fetch)
origin	https://github.com/tektoncd/pipeline.git (push)
`,
			want: RepoRef{Owner: "tektoncd", Repo: "pipeline"},
		},
		{
			name: "multiple remotes prefers origin",
			output: `upstream	git@github.com:other/repo.git (fetch)
upstream	git@github.com:other/repo.git (push)
origin	git@github.com:myuser/myrepo.git (fetch)
origin	git@github.com:myuser/myrepo.git (push)
`,
			want: RepoRef{Owner: "myuser", Repo: "myrepo"},
		},
		{
			name: "no origin falls back to first GitHub remote",
			output: `upstream	git@github.com:tektoncd/pipeline.git (fetch)
upstream	git@github.com:tektoncd/pipeline.git (push)
`,
			want: RepoRef{Owner: "tektoncd", Repo: "pipeline"},
		},
		{
			name:    "no GitHub remotes",
			output:  `origin	git@gitlab.com:owner/repo.git (fetch)`,
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitRemoteOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitRemoteOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseGitRemoteOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
