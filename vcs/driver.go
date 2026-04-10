package vcs

import "context"

// RepoStatus represents the VCS status of a repository.
type RepoStatus struct {
	Branch    string   `json:"branch"`
	Dirty     bool     `json:"dirty"`
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
	Modified  []string `json:"modified,omitempty"`
	Untracked []string `json:"untracked,omitempty"`
	Staged    []string `json:"staged,omitempty"`
}

// Commit represents a VCS commit.
type Commit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// GrepResult represents a single grep match.
type GrepResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Worktree represents a VCS worktree.
type Worktree struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// Driver is the interface for VCS operations.
// Both git and Sapling implement this interface.
type Driver interface {
	// Name returns the driver name ("git" or "sl").
	Name() string

	// Clone clones a repository to the given path.
	Clone(ctx context.Context, url, path, branch string) error

	// Pull pulls the current branch from the remote.
	Pull(ctx context.Context, path string) error

	// Push pushes the current branch to the remote.
	Push(ctx context.Context, path string) error

	// Fetch fetches from the remote without merging.
	Fetch(ctx context.Context, path string) error

	// Checkout switches to the given branch.
	// If create is true, creates the branch first.
	Checkout(ctx context.Context, path, branch string, create bool) error

	// Status returns the current status of the repository.
	Status(ctx context.Context, path string) (*RepoStatus, error)

	// Commit creates a commit with the given message.
	// If all is true, stages all modified files first.
	Commit(ctx context.Context, path, message string, all bool) error

	// CurrentBranch returns the name of the current branch.
	CurrentBranch(ctx context.Context, path string) (string, error)

	// Log returns the last n commits.
	Log(ctx context.Context, path string, limit int) ([]Commit, error)

	// Diff returns the diff of uncommitted changes.
	Diff(ctx context.Context, path string) (string, error)

	// Grep searches for a pattern in the repository.
	Grep(ctx context.Context, path, pattern string) ([]GrepResult, error)

	// WorktreeAdd creates a new worktree.
	// Returns the path to the new worktree.
	WorktreeAdd(ctx context.Context, path, name, branch string) (string, error)

	// WorktreeList lists all worktrees.
	WorktreeList(ctx context.Context, path string) ([]Worktree, error)

	// WorktreeRemove removes a worktree.
	WorktreeRemove(ctx context.Context, path, name string) error
}
