package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_Valid(t *testing.T) {
	yaml := `
version: "1"
workspace:
  name: test-workspace
  description: A test workspace
repos:
  repo-a:
    url: git@github.com:org/repo-a.git
    branch: main
  repo-b:
    url: git@github.com:org/repo-b.git
    path: custom-path
    branch: develop
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("expected version '1', got %q", cfg.Version)
	}
	if cfg.Workspace.Name != "test-workspace" {
		t.Errorf("expected workspace name 'test-workspace', got %q", cfg.Workspace.Name)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}

	repoA := cfg.Repos["repo-a"]
	if repoA.URL != "git@github.com:org/repo-a.git" {
		t.Errorf("repo-a url: got %q", repoA.URL)
	}
	if repoA.Branch != "main" {
		t.Errorf("repo-a branch: got %q", repoA.Branch)
	}

	repoB := cfg.Repos["repo-b"]
	if repoB.Path != "custom-path" {
		t.Errorf("repo-b path: got %q", repoB.Path)
	}
}

func TestParse_DefaultVersion(t *testing.T) {
	yaml := `
workspace:
  name: test
repos:
  r:
    url: git@github.com:org/r.git
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("expected default version '1', got %q", cfg.Version)
	}
}

func TestParse_MissingWorkspaceName(t *testing.T) {
	yaml := `
version: "1"
workspace:
  description: no name
repos:
  r:
    url: git@github.com:org/r.git
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing workspace name")
	}
}

func TestParse_MissingRepoURL(t *testing.T) {
	yaml := `
version: "1"
workspace:
  name: test
repos:
  r:
    branch: main
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing repo URL")
	}
}

func TestParse_NoRepos(t *testing.T) {
	yaml := `
version: "1"
workspace:
  name: test
repos: {}
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty repos")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("{{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestResolve_Defaults(t *testing.T) {
	repo := Repo{URL: "git@github.com:org/r.git"}
	resolved := repo.Resolve("my-repo")

	if resolved.Name != "my-repo" {
		t.Errorf("name: got %q", resolved.Name)
	}
	if resolved.Path != "my-repo" {
		t.Errorf("path should default to name, got %q", resolved.Path)
	}
	if resolved.Branch != "main" {
		t.Errorf("branch should default to 'main', got %q", resolved.Branch)
	}
}

func TestResolve_ExplicitValues(t *testing.T) {
	repo := Repo{
		URL:    "git@github.com:org/r.git",
		Path:   "custom",
		Branch: "develop",
	}
	resolved := repo.Resolve("my-repo")

	if resolved.Path != "custom" {
		t.Errorf("path: got %q", resolved.Path)
	}
	if resolved.Branch != "develop" {
		t.Errorf("branch: got %q", resolved.Branch)
	}
}

func TestResolveAll(t *testing.T) {
	cfg := &Config{
		Workspace: WorkspaceConfig{Name: "test"},
		Repos: map[string]Repo{
			"a": {URL: "url-a"},
			"b": {URL: "url-b", Path: "b-path"},
		},
	}
	repos := cfg.ResolveAll()
	if len(repos) != 2 {
		t.Fatalf("expected 2 resolved repos, got %d", len(repos))
	}
}

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Version:   "1",
		Workspace: WorkspaceConfig{Name: "test", Description: "test workspace"},
		Repos: map[string]Repo{
			"r": {URL: "git@github.com:org/r.git", Branch: "main"},
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Workspace.Name != "test" {
		t.Errorf("loaded workspace name: got %q", loaded.Workspace.Name)
	}
	if len(loaded.Repos) != 1 {
		t.Errorf("loaded repos count: got %d", len(loaded.Repos))
	}
}

func TestFindRoot(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(sub, 0755)

	// Place mit.yaml at dir level
	os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`
version: "1"
workspace:
  name: test
repos:
  r:
    url: url
`), 0644)

	root, err := FindRoot(sub)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if root != dir {
		t.Errorf("expected root %q, got %q", dir, root)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRoot(dir)
	if err == nil {
		t.Fatal("expected error when mit.yaml not found")
	}
}
