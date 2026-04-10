package forge

import "testing"

func TestForForgeType(t *testing.T) {
	tests := []struct {
		ft      ForgeType
		wantErr bool
	}{
		{GitHub, false},
		{GitLab, false},
		{"bitbucket", true},
		{"", true},
	}

	for _, tt := range tests {
		f, err := ForForgeType(tt.ft)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ForForgeType(%q): expected error", tt.ft)
			}
			continue
		}
		if err != nil {
			t.Errorf("ForForgeType(%q): unexpected error: %v", tt.ft, err)
			continue
		}
		if f.Type() != tt.ft {
			t.Errorf("ForForgeType(%q).Type() = %q", tt.ft, f.Type())
		}
	}
}

func TestResolveForge(t *testing.T) {
	tests := []struct {
		name      string
		repoURL   string
		forgeStr  string
		wantOwner string
		wantRepo  string
		wantType  ForgeType
		wantErr   bool
	}{
		{
			name:      "github-https",
			repoURL:   "https://github.com/org/repo.git",
			forgeStr:  "github",
			wantOwner: "org",
			wantRepo:  "repo",
			wantType:  GitHub,
		},
		{
			name:      "github-ssh",
			repoURL:   "git@github.com:org/repo.git",
			forgeStr:  "github",
			wantOwner: "org",
			wantRepo:  "repo",
			wantType:  GitHub,
		},
		{
			name:      "gitlab-ssh",
			repoURL:   "git@gitlab.com:team/project.git",
			forgeStr:  "gitlab",
			wantOwner: "team",
			wantRepo:  "project",
			wantType:  GitLab,
		},
		{
			name:      "gitlab-nested-groups",
			repoURL:   "git@gitlab.com:group/subgroup/repo.git",
			forgeStr:  "gitlab",
			wantOwner: "group/subgroup",
			wantRepo:  "repo",
			wantType:  GitLab,
		},
		{
			name:      "gitlab-self-hosted",
			repoURL:   "git@gitlab.example.com:acme/web-api.git",
			forgeStr:  "gitlab",
			wantOwner: "acme",
			wantRepo:  "web-api",
			wantType:  GitLab,
		},
		{
			name:      "github-enterprise",
			repoURL:   "git@github.internal:team/repo.git",
			forgeStr:  "github",
			wantOwner: "team",
			wantRepo:  "repo",
			wantType:  GitHub,
		},
		{
			name:     "empty-forge",
			repoURL:  "git@github.com:org/repo.git",
			forgeStr: "",
			wantErr:  true,
		},
		{
			name:     "unknown-forge",
			repoURL:  "git@github.com:org/repo.git",
			forgeStr: "bitbucket",
			wantErr:  true,
		},
		{
			name:     "bad-url",
			repoURL:  "not-a-url",
			forgeStr: "github",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, owner, repo, err := ResolveForge(tt.name, tt.repoURL, tt.forgeStr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.Type() != tt.wantType {
				t.Errorf("type = %q, want %q", f.Type(), tt.wantType)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}
