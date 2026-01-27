package lazypr

import (
	"testing"
)

func TestParsePRRef_GitHubURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PRRef
		wantErr bool
	}{
		{
			name:  "standard GitHub URL",
			input: "https://github.com/owner/repo/pull/123",
			want:  PRRef{Owner: "owner", Repo: "repo", Number: 123},
		},
		{
			name:  "GitHub URL with http",
			input: "http://github.com/owner/repo/pull/456",
			want:  PRRef{Owner: "owner", Repo: "repo", Number: 456},
		},
		{
			name:  "GitHub URL with trailing whitespace",
			input: "  https://github.com/tektoncd/pipeline/pull/1234  ",
			want:  PRRef{Owner: "tektoncd", Repo: "pipeline", Number: 1234},
		},
		{
			name:  "GitHub URL with hyphenated names",
			input: "https://github.com/my-org/my-repo/pull/999",
			want:  PRRef{Owner: "my-org", Repo: "my-repo", Number: 999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePRRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePRRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePRRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePRRef_ShortFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PRRef
		wantErr bool
	}{
		{
			name:  "simple short format",
			input: "owner/repo#123",
			want:  PRRef{Owner: "owner", Repo: "repo", Number: 123},
		},
		{
			name:  "hyphenated names",
			input: "my-org/my-repo#456",
			want:  PRRef{Owner: "my-org", Repo: "my-repo", Number: 456},
		},
		{
			name:  "tekton example",
			input: "tektoncd/pipeline#1234",
			want:  PRRef{Owner: "tektoncd", Repo: "pipeline", Number: 1234},
		},
		{
			name:  "with whitespace",
			input: "  owner/repo#789  ",
			want:  PRRef{Owner: "owner", Repo: "repo", Number: 789},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePRRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePRRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePRRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePRRef_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "just number", input: "123"},
		{name: "missing number", input: "owner/repo#"},
		{name: "missing hash", input: "owner/repo"},
		{name: "invalid URL", input: "https://gitlab.com/owner/repo/pull/123"},
		{name: "random text", input: "not a pr reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePRRef(tt.input)
			if err == nil {
				t.Errorf("ParsePRRef(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestParsePRRefs(t *testing.T) {
	inputs := []string{
		"https://github.com/owner/repo1/pull/1",
		"owner/repo2#2",
		"https://github.com/owner/repo3/pull/3",
	}

	refs, err := ParsePRRefs(inputs)
	if err != nil {
		t.Fatalf("ParsePRRefs() error = %v", err)
	}

	if len(refs) != 3 {
		t.Fatalf("ParsePRRefs() got %d refs, want 3", len(refs))
	}

	expected := []PRRef{
		{Owner: "owner", Repo: "repo1", Number: 1},
		{Owner: "owner", Repo: "repo2", Number: 2},
		{Owner: "owner", Repo: "repo3", Number: 3},
	}

	for i, ref := range refs {
		if ref != expected[i] {
			t.Errorf("refs[%d] = %v, want %v", i, ref, expected[i])
		}
	}
}

func TestPRRef_String(t *testing.T) {
	ref := PRRef{Owner: "tektoncd", Repo: "pipeline", Number: 1234}
	got := ref.String()
	want := "tektoncd/pipeline#1234"
	if got != want {
		t.Errorf("PRRef.String() = %q, want %q", got, want)
	}
}

func TestPRRef_URL(t *testing.T) {
	ref := PRRef{Owner: "tektoncd", Repo: "pipeline", Number: 1234}
	got := ref.URL()
	want := "https://github.com/tektoncd/pipeline/pull/1234"
	if got != want {
		t.Errorf("PRRef.URL() = %q, want %q", got, want)
	}
}
