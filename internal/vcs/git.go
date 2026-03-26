package vcs

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitDriver implements Driver for git.
type GitDriver struct{}

func NewGitDriver() *GitDriver {
	return &GitDriver{}
}

func (g *GitDriver) Name() string { return "git" }

func (g *GitDriver) Clone(ctx context.Context, url, path, branch string) error {
	args := []string{"clone", url, path}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	return g.run(ctx, "", args...)
}

func (g *GitDriver) Pull(ctx context.Context, path string) error {
	return g.run(ctx, path, "pull", "--ff-only")
}

func (g *GitDriver) Push(ctx context.Context, path string) error {
	return g.run(ctx, path, "push")
}

func (g *GitDriver) Fetch(ctx context.Context, path string) error {
	return g.run(ctx, path, "fetch", "--all", "--prune")
}

func (g *GitDriver) Checkout(ctx context.Context, path, branch string, create bool) error {
	if create {
		return g.run(ctx, path, "checkout", "-b", branch)
	}
	return g.run(ctx, path, "checkout", branch)
}

func (g *GitDriver) Status(ctx context.Context, path string) (*RepoStatus, error) {
	status := &RepoStatus{}

	// Get current branch
	branch, err := g.CurrentBranch(ctx, path)
	if err != nil {
		return nil, err
	}
	status.Branch = branch

	// Get porcelain status
	out, err := g.output(ctx, path, "status", "--porcelain", "-b")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if strings.HasPrefix(line, "##") {
			// Parse branch tracking info for ahead/behind
			if idx := strings.Index(line, "["); idx != -1 {
				tracking := line[idx:]
				if strings.Contains(tracking, "ahead") {
					parts := strings.Split(tracking, "ahead ")
					if len(parts) > 1 {
						n, _ := strconv.Atoi(strings.TrimRight(strings.Split(parts[1], ",")[0], "]"))
						status.Ahead = n
					}
				}
				if strings.Contains(tracking, "behind") {
					parts := strings.Split(tracking, "behind ")
					if len(parts) > 1 {
						n, _ := strconv.Atoi(strings.TrimRight(strings.Split(parts[1], ",")[0], "]"))
						status.Behind = n
					}
				}
			}
			continue
		}

		x := line[0]
		y := line[1]
		file := strings.TrimSpace(line[2:])

		if x == '?' && y == '?' {
			status.Untracked = append(status.Untracked, file)
			status.Dirty = true
		} else {
			if x != ' ' && x != '?' {
				status.Staged = append(status.Staged, file)
				status.Dirty = true
			}
			if y != ' ' && y != '?' {
				status.Modified = append(status.Modified, file)
				status.Dirty = true
			}
		}
	}

	return status, nil
}

func (g *GitDriver) Commit(ctx context.Context, path, message string, all bool) error {
	args := []string{"commit", "-m", message}
	if all {
		args = append(args, "-a")
	}
	return g.run(ctx, path, args...)
}

func (g *GitDriver) CurrentBranch(ctx context.Context, path string) (string, error) {
	out, err := g.output(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (g *GitDriver) Log(ctx context.Context, path string, limit int) ([]Commit, error) {
	format := "--format=%H\x1f%an\x1f%aI\x1f%s"
	out, err := g.output(ctx, path, "log", format, fmt.Sprintf("-n%d", limit))
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
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

func (g *GitDriver) Diff(ctx context.Context, path string) (string, error) {
	return g.output(ctx, path, "diff")
}

func (g *GitDriver) Grep(ctx context.Context, path, pattern string) ([]GrepResult, error) {
	out, err := g.outputRaw(ctx, path, "grep", "-n", "--no-color", pattern)
	if err != nil {
		return nil, fmt.Errorf("git grep: %w", err)
	}

	var results []GrepResult
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		// Format: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum, _ := strconv.Atoi(parts[1])
		results = append(results, GrepResult{
			File:    parts[0],
			Line:    lineNum,
			Content: parts[2],
		})
	}
	return results, nil
}

func (g *GitDriver) WorktreeAdd(ctx context.Context, path, name, branch string) (string, error) {
	wtPath := filepath.Join(filepath.Dir(path), ".mit-worktrees", name, filepath.Base(path))
	args := []string{"worktree", "add", wtPath}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	if err := g.run(ctx, path, args...); err != nil {
		return "", fmt.Errorf("git worktree add: %w", err)
	}
	return wtPath, nil
}

func (g *GitDriver) WorktreeList(ctx context.Context, path string) ([]Worktree, error) {
	out, err := g.output(ctx, path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []Worktree
	var current Worktree
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
			current.Name = filepath.Base(current.Path)
		}
		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	return worktrees, nil
}

func (g *GitDriver) WorktreeRemove(ctx context.Context, path, name string) error {
	wtPath := filepath.Join(filepath.Dir(path), ".mit-worktrees", name, filepath.Base(path))
	return g.run(ctx, path, "worktree", "remove", "--force", wtPath)
}

// run executes a git command.
func (g *GitDriver) run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// outputRaw executes a git command, treating exit code 1 as "no results" (empty string, no error).
// This is useful for commands like grep that return exit code 1 when no matches found.
func (g *GitDriver) outputRaw(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", err
	}
	return string(out), nil
}

// output executes a git command and returns stdout.
func (g *GitDriver) output(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", err
	}
	return string(out), nil
}
