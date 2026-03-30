package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitHubForge implements Forge using the gh CLI.
type GitHubForge struct {
	Host string // e.g., "github.com" or a GitHub Enterprise host
}

func (g *GitHubForge) Type() ForgeType { return GitHub }

func (g *GitHubForge) CheckAvailable() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh CLI not found; install from https://cli.github.com")
	}
	return nil
}

func (g *GitHubForge) CheckAuthenticated() error {
	if err := g.CheckAvailable(); err != nil {
		return err
	}
	args := []string{"auth", "status"}
	if g.Host != "" && g.Host != "github.com" {
		args = append(args, "--hostname", g.Host)
	}
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		host := g.Host
		if host == "" {
			host = "github.com"
		}
		return fmt.Errorf("gh CLI not authenticated for %s; run 'gh auth login --hostname %s'\n%s",
			host, host, strings.TrimSpace(string(out)))
	}
	return nil
}

// gh issue list JSON fields
const ghIssueFields = "number,title,body,state,url,labels,author,createdAt"

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	Labels    []ghLabel `json:"labels"`
	Author    ghAuthor  `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

func (gi *ghIssue) toIssue() Issue {
	labels := make([]string, len(gi.Labels))
	for i, l := range gi.Labels {
		labels[i] = l.Name
	}
	return Issue{
		Number:    gi.Number,
		Title:     gi.Title,
		Body:      gi.Body,
		State:     gi.State,
		URL:       gi.URL,
		Labels:    labels,
		Author:    gi.Author.Login,
		CreatedAt: gi.CreatedAt,
	}
}

func (g *GitHubForge) ListIssues(ctx context.Context, owner, repo, state string) ([]Issue, error) {
	if state == "" {
		state = "open"
	}
	out, err := g.output(ctx, "issue", "list",
		"--repo", owner+"/"+repo,
		"--state", state,
		"--json", ghIssueFields,
		"--limit", "50",
	)
	if err != nil {
		return nil, err
	}

	var raw []ghIssue
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh issue list output: %w", err)
	}

	issues := make([]Issue, len(raw))
	for i, r := range raw {
		issues[i] = r.toIssue()
	}
	return issues, nil
}

func (g *GitHubForge) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	out, err := g.output(ctx, "issue", "view",
		fmt.Sprintf("%d", number),
		"--repo", owner+"/"+repo,
		"--json", ghIssueFields,
	)
	if err != nil {
		return nil, err
	}

	var raw ghIssue
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh issue view output: %w", err)
	}

	issue := raw.toIssue()
	return &issue, nil
}

func (g *GitHubForge) CreateIssue(ctx context.Context, owner, repo, title, body string) (*Issue, error) {
	args := []string{"issue", "create",
		"--repo", owner + "/" + repo,
		"--title", title,
	}
	if body != "" {
		args = append(args, "--body", body)
	}

	out, err := g.output(ctx, args...)
	if err != nil {
		return nil, err
	}

	// gh issue create returns the URL of the created issue
	url := strings.TrimSpace(out)
	return &Issue{
		Title: title,
		Body:  body,
		State: "open",
		URL:   url,
	}, nil
}

func (g *GitHubForge) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error {
	_, err := g.output(ctx, "issue", "comment",
		fmt.Sprintf("%d", number),
		"--repo", owner+"/"+repo,
		"--body", body,
	)
	return err
}

// Diff methods — not yet implemented

func (g *GitHubForge) ListDiffs(ctx context.Context, owner, repo, state string) ([]Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitHubForge) GetDiff(ctx context.Context, owner, repo string, number int) (*Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitHubForge) CreateDiff(ctx context.Context, owner, repo, title, body, head, base string) (*Diff, error) {
	return nil, ErrNotImplemented
}

func (g *GitHubForge) ListDiffComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	return nil, ErrNotImplemented
}

func (g *GitHubForge) CommentOnDiff(ctx context.Context, owner, repo string, number int, body string) error {
	return ErrNotImplemented
}

func (g *GitHubForge) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	// For GitHub Enterprise, set GH_HOST so gh targets the right server.
	if g.Host != "" && g.Host != "github.com" {
		cmd.Env = appendEnv(cmd, "GH_HOST="+g.Host)
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh %s: %s", strings.Join(args[:2], " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("gh %s: %w", strings.Join(args[:2], " "), err)
	}
	return string(out), nil
}
