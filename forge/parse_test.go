package forge

import "testing"

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		// HTTPS
		{"https://github.com/org/repo.git", "org", "repo", false},
		{"https://github.com/org/repo", "org", "repo", false},
		{"https://gitlab.com/org/repo.git", "org", "repo", false},
		{"https://gitlab.internal/team/project.git", "team", "project", false},

		// SSH
		{"git@github.com:org/repo.git", "org", "repo", false},
		{"git@github.com:org/repo", "org", "repo", false},
		{"git@gitlab.com:org/repo.git", "org", "repo", false},
		{"git@gitlab.internal:team/project.git", "team", "project", false},

		// Nested groups (GitLab subgroups)
		{"https://gitlab.com/group/subgroup/repo.git", "group/subgroup", "repo", false},
		{"git@gitlab.com:group/subgroup/repo.git", "group/subgroup", "repo", false},
		{"git@gitlab.com:a/b/c/repo.git", "a/b/c", "repo", false},

		// Trailing slash
		{"https://github.com/org/repo/", "org", "repo", false},

		// Errors
		{"", "", "", true},
		{"just-a-name", "", "", true},
		{"https://github.com/", "", "", true},
		{"git@github.com:", "", "", true},
	}

	for _, tt := range tests {
		owner, repo, err := ParseOwnerRepo(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseOwnerRepo(%q): expected error", tt.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseOwnerRepo(%q): unexpected error: %v", tt.url, err)
			continue
		}
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseOwnerRepo(%q) = (%q, %q), want (%q, %q)", tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestParseHostOwnerRepo(t *testing.T) {
	tests := []struct {
		url       string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https://github.com/org/repo.git", "github.com", "org", "repo", false},
		{"git@github.com:org/repo.git", "github.com", "org", "repo", false},
		{"git@gitlab.example.com:acme/web-api.git", "gitlab.example.com", "acme", "web-api", false},
		{"https://gitlab.example.com/acme/web-api.git", "gitlab.example.com", "acme", "web-api", false},
		{"git@gitlab.com:group/subgroup/repo.git", "gitlab.com", "group/subgroup", "repo", false},
		{"", "", "", "", true},
	}

	for _, tt := range tests {
		host, owner, repo, err := ParseHostOwnerRepo(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseHostOwnerRepo(%q): expected error", tt.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseHostOwnerRepo(%q): unexpected error: %v", tt.url, err)
			continue
		}
		if host != tt.wantHost || owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseHostOwnerRepo(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.url, host, owner, repo, tt.wantHost, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestFormatRemoteID(t *testing.T) {
	if got := FormatRemoteID(GitHub, 42); got != "github#42" {
		t.Errorf("FormatRemoteID(GitHub, 42) = %q", got)
	}
	if got := FormatRemoteID(GitLab, 7); got != "gitlab#7" {
		t.Errorf("FormatRemoteID(GitLab, 7) = %q", got)
	}
}

func TestParseRemoteID(t *testing.T) {
	tests := []struct {
		id      string
		wantFT  ForgeType
		wantN   int
		wantOK  bool
	}{
		{"github#42", GitHub, 42, true},
		{"gitlab#7", GitLab, 7, true},
		{"github#0", "", 0, false},
		{"github#-1", "", 0, false},
		{"github#abc", "", 0, false},
		{"unknown#42", "", 0, false},
		{"abc-def-123", "", 0, false},
		{"", "", 0, false},
	}

	for _, tt := range tests {
		ft, n, ok := ParseRemoteID(tt.id)
		if ok != tt.wantOK {
			t.Errorf("ParseRemoteID(%q): ok = %v, want %v", tt.id, ok, tt.wantOK)
			continue
		}
		if ok && (ft != tt.wantFT || n != tt.wantN) {
			t.Errorf("ParseRemoteID(%q) = (%q, %d), want (%q, %d)", tt.id, ft, n, tt.wantFT, tt.wantN)
		}
	}
}
