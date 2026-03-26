package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/vcs"
)

// Workspace represents a loaded mit workspace.
type Workspace struct {
	Root   string
	Config *config.Config
	Repos  []RepoInfo
}

// RepoInfo holds resolved repo info with its driver.
type RepoInfo struct {
	config.ResolvedRepo
	AbsPath string
	Driver  vcs.Driver
	Exists  bool
}

// Load loads a workspace from the given directory (or any child directory).
func Load(dir string) (*Workspace, error) {
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}

	ws := &Workspace{
		Root:   root,
		Config: cfg,
	}

	for _, resolved := range cfg.ResolveAll() {
		absPath := filepath.Join(root, resolved.Path)
		info := RepoInfo{
			ResolvedRepo: resolved,
			AbsPath:      absPath,
			Exists:       dirExists(absPath),
		}

		// Auto-detect VCS driver if repo exists
		if info.Exists {
			driver, err := vcs.Detect(absPath)
			if err == nil {
				info.Driver = driver
			}
		}

		ws.Repos = append(ws.Repos, info)
	}

	return ws, nil
}

// FilterRepos filters the workspace repos using the selector.
func (ws *Workspace) FilterRepos(sel *Selector) []RepoInfo {
	if sel == nil || sel.IsEmpty() {
		return ws.Repos
	}

	var filtered []RepoInfo
	for _, repo := range ws.Repos {
		if sel.Matches(repo.Name) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

// GetRepo returns a single repo by name.
func (ws *Workspace) GetRepo(name string) (*RepoInfo, error) {
	for _, repo := range ws.Repos {
		if repo.Name == name {
			return &repo, nil
		}
	}
	return nil, fmt.Errorf("repo %q not found in workspace", name)
}

// RepoNames returns the names of all repos.
func (ws *Workspace) RepoNames() []string {
	names := make([]string, len(ws.Repos))
	for i, r := range ws.Repos {
		names[i] = r.Name
	}
	return names
}

// EnsureDriver returns the repo's VCS driver, or detects/creates one.
func (ws *Workspace) EnsureDriver(ctx context.Context, repo *RepoInfo, defaultVCS string) (vcs.Driver, error) {
	if repo.Driver != nil {
		return repo.Driver, nil
	}
	return vcs.DetectOrDefault(repo.AbsPath, defaultVCS)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
