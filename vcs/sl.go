package vcs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SaplingDriver implements Driver for Sapling (sl).
type SaplingDriver struct{}

func NewSaplingDriver() *SaplingDriver {
	return &SaplingDriver{}
}

func (s *SaplingDriver) Name() string { return "sl" }

func (s *SaplingDriver) Clone(ctx context.Context, url, path, branch string) error {
	args := []string{"clone", url, path}
	if branch != "" {
		args = append(args, "--updaterev", branch)
	}
	return s.run(ctx, "", args...)
}

func (s *SaplingDriver) Pull(ctx context.Context, path string) error {
	// sl pull --update fetches and updates the working copy (like git pull)
	return s.run(ctx, path, "pull", "--update")
}

func (s *SaplingDriver) Push(ctx context.Context, path string) error {
	// Sapling pushes to a specific bookmark
	branch, err := s.CurrentBranch(ctx, path)
	if err != nil {
		return err
	}
	return s.run(ctx, path, "push", "--to", branch)
}

func (s *SaplingDriver) Fetch(ctx context.Context, path string) error {
	// Sapling's pull is equivalent to git fetch (no merge)
	return s.run(ctx, path, "pull")
}

func (s *SaplingDriver) Checkout(ctx context.Context, path, branch string, create bool) error {
	if create {
		// In Sapling, create a bookmark (branch equivalent)
		if err := s.run(ctx, path, "bookmark", branch); err != nil {
			return err
		}
	}
	return s.run(ctx, path, "goto", branch)
}

func (s *SaplingDriver) Status(ctx context.Context, path string) (*RepoStatus, error) {
	status := &RepoStatus{}

	// Get current branch/bookmark
	branch, err := s.CurrentBranch(ctx, path)
	if err != nil {
		return nil, err
	}
	status.Branch = branch

	// Get status output
	out, err := s.output(ctx, path, "status")
	if err != nil {
		return nil, fmt.Errorf("sl status: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if len(line) < 2 {
			continue
		}
		marker := line[0]
		file := strings.TrimSpace(line[2:])

		switch marker {
		case 'M':
			status.Modified = append(status.Modified, file)
			status.Dirty = true
		case 'A':
			status.Staged = append(status.Staged, file)
			status.Dirty = true
		case 'R':
			// Removed
			status.Modified = append(status.Modified, file)
			status.Dirty = true
		case '?':
			status.Untracked = append(status.Untracked, file)
			status.Dirty = true
		case '!':
			// Missing
			status.Modified = append(status.Modified, file)
			status.Dirty = true
		}
	}

	// Ahead/behind not directly available in sl status,
	// would need sl log to calculate. Leave as 0 for now.

	return status, nil
}

func (s *SaplingDriver) Commit(ctx context.Context, path, message string, all bool) error {
	// Sapling commits all modified files by default (no staging area).
	// --addremove includes untracked files when 'all' is true.
	args := []string{"commit", "-m", message}
	if all {
		args = append(args, "--addremove")
	}
	return s.run(ctx, path, args...)
}

func (s *SaplingDriver) CurrentBranch(ctx context.Context, path string) (string, error) {
	// Sapling uses bookmarks as branches
	out, err := s.output(ctx, path, "log", "-r", ".", "--template", "{activebookmark}")
	if err != nil {
		return "", fmt.Errorf("getting current bookmark: %w", err)
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		// No active bookmark, use the commit hash
		out, err = s.output(ctx, path, "log", "-r", ".", "--template", "{node|short}")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
	return branch, nil
}

func (s *SaplingDriver) Log(ctx context.Context, path string, limit int) ([]Commit, error) {
	template := "{node|short}\x1f{author|person}\x1f{date|isodate}\x1f{desc|firstline}\n"
	out, err := s.output(ctx, path, "log", "--template", template, "--limit", fmt.Sprintf("%d", limit))
	if err != nil {
		return nil, fmt.Errorf("sl log: %w", err)
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}
	return commits, nil
}

func (s *SaplingDriver) Diff(ctx context.Context, path string) (string, error) {
	return s.output(ctx, path, "diff")
}

func (s *SaplingDriver) Grep(ctx context.Context, path, pattern string) ([]GrepResult, error) {
	out, err := s.output(ctx, path, "grep", "-n", pattern)
	if err != nil {
		// sl grep returns exit code 1 when no matches
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("sl grep: %w", err)
	}

	var results []GrepResult
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		// Sapling grep format: file:rev:line:content
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			// Try file:line:content format
			parts = strings.SplitN(line, ":", 3)
			if len(parts) < 3 {
				continue
			}
			lineNum, _ := strconv.Atoi(parts[1])
			results = append(results, GrepResult{
				File:    parts[0],
				Line:    lineNum,
				Content: parts[2],
			})
			continue
		}
		lineNum, _ := strconv.Atoi(parts[2])
		results = append(results, GrepResult{
			File:    parts[0],
			Line:    lineNum,
			Content: parts[3],
		})
	}
	return results, nil
}

// WorktreeAdd for Sapling falls back to cloning since sl doesn't support worktrees.
func (s *SaplingDriver) WorktreeAdd(ctx context.Context, path, name, branch string) (string, error) {
	wtPath := filepath.Join(filepath.Dir(path), ".mit-worktrees", name, filepath.Base(path))
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	// Clone the repo to the worktree path
	if err := s.Clone(ctx, path, wtPath, branch); err != nil {
		return "", fmt.Errorf("sl clone for worktree: %w", err)
	}

	return wtPath, nil
}

func (s *SaplingDriver) WorktreeList(ctx context.Context, path string) ([]Worktree, error) {
	// Sapling doesn't have native worktree support.
	// Check .mit-worktrees directory for cloned worktrees.
	wtBase := filepath.Join(filepath.Dir(path), ".mit-worktrees")
	entries, err := os.ReadDir(wtBase)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var worktrees []Worktree
	repoName := filepath.Base(path)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(wtBase, entry.Name(), repoName)
		if _, err := os.Stat(filepath.Join(wtPath, ".sl")); err != nil {
			continue
		}
		branch, _ := s.CurrentBranch(ctx, wtPath)
		worktrees = append(worktrees, Worktree{
			Name:   entry.Name(),
			Path:   wtPath,
			Branch: branch,
		})
	}
	return worktrees, nil
}

func (s *SaplingDriver) WorktreeRemove(ctx context.Context, path, name string) error {
	wtPath := filepath.Join(filepath.Dir(path), ".mit-worktrees", name, filepath.Base(path))
	return os.RemoveAll(wtPath)
}

func (s *SaplingDriver) run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "sl", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *SaplingDriver) output(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "sl", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("sl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", err
	}
	return string(out), nil
}
