package vcs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a temporary git repo with an initial commit.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "main"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	// Create initial file and commit
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	cmd.Run()

	return dir
}

func TestGitDriver_CurrentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	branch, err := driver.CurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected 'main', got %q", branch)
	}
}

func TestGitDriver_Status_Clean(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	status, err := driver.Status(ctx, dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Dirty {
		t.Error("expected clean repo")
	}
	if status.Branch != "main" {
		t.Errorf("branch: got %q", status.Branch)
	}
}

func TestGitDriver_Status_Dirty(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	// Create a new file (untracked)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	// Modify existing file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644)

	status, err := driver.Status(ctx, dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Dirty {
		t.Error("expected dirty repo")
	}
	if len(status.Untracked) == 0 {
		t.Error("expected untracked files")
	}
	if len(status.Modified) == 0 {
		t.Error("expected modified files")
	}
}

func TestGitDriver_Checkout(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	// Create a new branch
	if err := driver.Checkout(ctx, dir, "feature", true); err != nil {
		t.Fatalf("Checkout create: %v", err)
	}

	branch, _ := driver.CurrentBranch(ctx, dir)
	if branch != "feature" {
		t.Errorf("expected 'feature', got %q", branch)
	}

	// Switch back to main
	if err := driver.Checkout(ctx, dir, "main", false); err != nil {
		t.Fatalf("Checkout main: %v", err)
	}

	branch, _ = driver.CurrentBranch(ctx, dir)
	if branch != "main" {
		t.Errorf("expected 'main', got %q", branch)
	}
}

func TestGitDriver_Commit(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	// Modify an existing tracked file (git commit -a only stages tracked files)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Updated\n"), 0644)
	if err := driver.Commit(ctx, dir, "update readme", true); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	status, _ := driver.Status(ctx, dir)
	if status.Dirty {
		t.Error("expected clean after commit --all")
	}
}

func TestGitDriver_Log(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	commits, err := driver.Log(ctx, dir, 5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) < 1 {
		t.Fatal("expected at least 1 commit")
	}
	if commits[0].Message != "initial commit" {
		t.Errorf("expected 'initial commit', got %q", commits[0].Message)
	}
}

func TestGitDriver_Diff(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\n"), 0644)

	diff, err := driver.Diff(ctx, dir)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestGitDriver_Grep(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	results, err := driver.Grep(ctx, dir, "Test")
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected grep results for 'Test'")
	}
}

func TestGitDriver_Grep_NoMatch(t *testing.T) {
	dir := setupGitRepo(t)
	driver := NewGitDriver()
	ctx := context.Background()

	results, err := driver.Grep(ctx, dir, "nonexistent_pattern_xyz")
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestGitDriver_Clone(t *testing.T) {
	// Create a source repo
	src := setupGitRepo(t)
	dst := filepath.Join(t.TempDir(), "cloned")
	driver := NewGitDriver()
	ctx := context.Background()

	if err := driver.Clone(ctx, src, dst, "main"); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Verify the clone
	if _, err := os.Stat(filepath.Join(dst, ".git")); err != nil {
		t.Error("expected .git directory in clone")
	}
	if _, err := os.Stat(filepath.Join(dst, "README.md")); err != nil {
		t.Error("expected README.md in clone")
	}
}

func TestDetect_Git(t *testing.T) {
	dir := setupGitRepo(t)
	driver, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if driver.Name() != "git" {
		t.Errorf("expected git driver, got %q", driver.Name())
	}
}

func TestDetect_NoVCS(t *testing.T) {
	dir := t.TempDir()
	_, err := Detect(dir)
	if err == nil {
		t.Error("expected error for directory without VCS")
	}
}

func TestDriverByName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"git", "git", false},
		{"sl", "sl", false},
		{"svn", "", true},
	}
	for _, tt := range tests {
		driver, err := DriverByName(tt.name)
		if tt.wantErr {
			if err == nil {
				t.Errorf("DriverByName(%q): expected error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("DriverByName(%q): %v", tt.name, err)
			continue
		}
		if driver.Name() != tt.want {
			t.Errorf("DriverByName(%q).Name() = %q, want %q", tt.name, driver.Name(), tt.want)
		}
	}
}
