package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitLabForge implements Forge using the glab CLI.
type GitLabForge struct {
	Host string // e.g., "gitlab.com" or "gitlab.example.com"
}

func (g *GitLabForge) Type() ForgeType { return GitLab }

func (g *GitLabForge) CheckAvailable() error {
	_, err := exec.LookPath("glab")
	if err != nil {
		return fmt.Errorf("glab CLI not found; install from https://gitlab.com/gitlab-org/cli")
	}
	return nil
}

func (g *GitLabForge) CheckAuthenticated() error {
	if err := g.CheckAvailable(); err != nil {
		return err
	}
	args := []string{"auth", "status"}
	if g.Host != "" && g.Host != "gitlab.com" {
		args = append(args, "--hostname", g.Host)
	}
	cmd := exec.Command("glab", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		host := g.Host
		if host == "" {
			host = "gitlab.com"
		}
		return fmt.Errorf("glab CLI not authenticated for %s; run 'glab auth login --hostname %s'\n%s",
			host, host, strings.TrimSpace(string(out)))
	}
	return nil
}

type glabIssue struct {
	IID       int       `json:"iid"`
	Title     string    `json:"title"`
	Desc      string    `json:"description"`
	State     string    `json:"state"`
	WebURL    string    `json:"web_url"`
	Labels    []string  `json:"labels"`
	Author    glabUser  `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type glabUser struct {
	Username string `json:"username"`
}

func (gi *glabIssue) toIssue() Issue {
	return Issue{
		Number:    gi.IID,
		Title:     gi.Title,
		Body:      gi.Desc,
		State:     gi.State,
		URL:       gi.WebURL,
		Labels:    gi.Labels,
		Author:    gi.Author.Username,
		CreatedAt: gi.CreatedAt,
	}
}

func (g *GitLabForge) ListIssues(ctx context.Context, owner, repo, state string) ([]Issue, error) {
	if state == "" {
		state = "opened"
	}
	// glab uses "opened" not "open"
	if state == "open" {
		state = "opened"
	}

	out, err := g.output(ctx, "issue", "list",
		"--repo", owner+"/"+repo,
		"--per-page", "50",
		"--output", "json",
	)
	if err != nil {
		return nil, err
	}

	var raw []glabIssue
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing glab issue list output: %w", err)
	}

	issues := make([]Issue, len(raw))
	for i, r := range raw {
		issues[i] = r.toIssue()
	}
	return issues, nil
}

func (g *GitLabForge) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	out, err := g.output(ctx, "issue", "view",
		fmt.Sprintf("%d", number),
		"--repo", owner+"/"+repo,
		"--output", "json",
	)
	if err != nil {
		return nil, err
	}

	var raw glabIssue
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing glab issue view output: %w", err)
	}

	issue := raw.toIssue()
	return &issue, nil
}

func (g *GitLabForge) CreateIssue(ctx context.Context, owner, repo, title, body string) (*Issue, error) {
	args := []string{"issue", "create",
		"--repo", owner + "/" + repo,
		"--title", title,
	}
	if body != "" {
		args = append(args, "--description", body)
	}

	out, err := g.output(ctx, args...)
	if err != nil {
		return nil, err
	}

	url := strings.TrimSpace(out)
	return &Issue{
		Title: title,
		Body:  body,
		State: "opened",
		URL:   url,
	}, nil
}

func (g *GitLabForge) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error {
	_, err := g.output(ctx, "issue", "note",
		fmt.Sprintf("%d", number),
		"--repo", owner+"/"+repo,
		"--message", body,
	)
	return err
}

// Diff methods — not yet implemented

func (g *GitLabForge) ListDiffs(ctx context.Context, owner, repo, state string) ([]Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitLabForge) GetDiff(ctx context.Context, owner, repo string, number int) (*Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitLabForge) CreateDiff(ctx context.Context, owner, repo, title, body, head, base string) (*Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitLabForge) ListDiffComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	return nil, ErrNotImplemented
}

func (g *GitLabForge) CommentOnDiff(ctx context.Context, owner, repo string, number int, body string) error {
	return ErrNotImplemented
}

func (g *GitLabForge) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	// For self-hosted instances, set GITLAB_HOST so glab targets the right server.
	if g.Host != "" && g.Host != "gitlab.com" {
		cmd.Env = appendEnv(cmd, "GITLAB_HOST="+g.Host)
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("glab %s: %s", strings.Join(args[:2], " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("glab %s: %w", strings.Join(args[:2], " "), err)
	}
	return string(out), nil
}
