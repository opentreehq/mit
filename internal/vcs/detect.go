package vcs

import (
	"fmt"
	"os"
	"path/filepath"
)

// Detect auto-detects the VCS driver for a repository path
// by checking for .git or .sl directories.
func Detect(path string) (Driver, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	gitDir := filepath.Join(absPath, ".git")
	if info, err := os.Stat(gitDir); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
		// .git can be a directory or a file (worktrees use a file pointing to the main repo)
		return NewGitDriver(), nil
	}

	slDir := filepath.Join(absPath, ".sl")
	if info, err := os.Stat(slDir); err == nil && info.IsDir() {
		return NewSaplingDriver(), nil
	}

	return nil, fmt.Errorf("no .git or .sl directory found in %s", absPath)
}

// DetectOrDefault tries to detect the VCS, falling back to the given default.
func DetectOrDefault(path, defaultVCS string) (Driver, error) {
	driver, err := Detect(path)
	if err == nil {
		return driver, nil
	}

	switch defaultVCS {
	case "git":
		return NewGitDriver(), nil
	case "sl":
		return NewSaplingDriver(), nil
	default:
		return nil, fmt.Errorf("unknown VCS driver %q", defaultVCS)
	}
}

// DriverByName returns a driver by name.
func DriverByName(name string) (Driver, error) {
	switch name {
	case "git":
		return NewGitDriver(), nil
	case "sl":
		return NewSaplingDriver(), nil
	default:
		return nil, fmt.Errorf("unknown VCS driver %q", name)
	}
}
