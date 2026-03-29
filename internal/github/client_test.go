package github

import (
	"testing"
)

func TestParseRepoFullName(t *testing.T) {
	tests := []struct {
		input    string
		owner    string
		repo     string
		wantErr  bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"org/sub-org/repo", "org", "sub-org/repo", false},
		{"a/b", "a", "b", false},
		{"", "", "", true},
		{"single", "", "", true},
		{"a/", "a", "", false},
	}

	for _, tt := range tests {
		owner, repo, err := ParseRepoFullName(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRepoFullName(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRepoFullName(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if owner != tt.owner || repo != tt.repo {
			t.Errorf("ParseRepoFullName(%q) = %q, %q; want %q, %q", tt.input, owner, repo, tt.owner, tt.repo)
		}
	}
}
