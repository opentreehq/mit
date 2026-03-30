package forge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ForgeType identifies the forge provider.
type ForgeType string

const (
	GitHub ForgeType = "github"
	GitLab ForgeType = "gitlab"
)

// ErrNotImplemented is returned by Forge methods that are not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// Issue represents a remote issue/ticket.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	Labels    []string  `json:"labels,omitempty"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Diff represents a remote pull request / merge request.
type Diff struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	Author    string    `json:"author,omitempty"`
	Branch    string    `json:"branch"`
	Base      string    `json:"base"`
	CreatedAt time.Time `json:"created_at"`
}

// Comment represents a comment on an issue or diff.
type Comment struct {
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Forge abstracts issue and diff operations on GitHub/GitLab.
type Forge interface {
	Type() ForgeType

	// Issues
	ListIssues(ctx context.Context, owner, repo, state string) ([]Issue, error)
	GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
	CreateIssue(ctx context.Context, owner, repo, title, body string) (*Issue, error)
	CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error

	// Diffs (pull requests / merge requests)
	ListDiffs(ctx context.Context, owner, repo, state string) ([]Diff, error)
	GetDiff(ctx context.Context, owner, repo string, number int) (*Diff, error)
	CreateDiff(ctx context.Context, owner, repo, title, body, head, base string) (*Diff, error)
	ListDiffComments(ctx context.Context, owner, repo string, number int) ([]Comment, error)
	CommentOnDiff(ctx context.Context, owner, repo string, number int, body string) error

	// Availability
	CheckAvailable() error
	CheckAuthenticated() error
}

// FormatRemoteID returns a display ID like "github#42".
func FormatRemoteID(ft ForgeType, number int) string {
	return fmt.Sprintf("%s#%d", ft, number)
}

// ParseRemoteID parses "github#42" or "gitlab#42" into forge type and number.
// Returns false if the ID doesn't match the remote pattern.
func ParseRemoteID(id string) (ForgeType, int, bool) {
	for _, ft := range []ForgeType{GitHub, GitLab} {
		prefix := string(ft) + "#"
		if strings.HasPrefix(id, prefix) {
			n, err := strconv.Atoi(id[len(prefix):])
			if err == nil && n > 0 {
				return ft, n, true
			}
		}
	}
	return "", 0, false
}

// appendEnv returns a copy of the current environment with extra vars appended.
// When cmd.Env is nil, exec uses the parent process environment; setting it
// requires copying os.Environ() first so we don't lose existing vars.
func appendEnv(cmd *exec.Cmd, extra ...string) []string {
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	return append(env, extra...)
}
