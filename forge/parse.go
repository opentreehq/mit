package forge

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseOwnerRepo extracts the owner and repo name from a git remote URL.
// Supports both SSH (git@host:owner/repo.git) and HTTPS (https://host/owner/repo.git) formats.
// For nested groups (e.g., GitLab subgroups), owner includes the full path (group/subgroup).
func ParseOwnerRepo(remoteURL string) (owner, repo string, err error) {
	_, owner, repo, err = ParseHostOwnerRepo(remoteURL)
	return
}

// ParseHostOwnerRepo extracts the host, owner, and repo name from a git remote URL.
// For self-hosted instances (e.g., gitlab.example.com), the host is needed
// so CLIs like glab can target the correct server.
func ParseHostOwnerRepo(remoteURL string) (host, owner, repo string, err error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", "", "", fmt.Errorf("empty remote URL")
	}

	var path string

	if strings.Contains(remoteURL, "://") {
		// HTTPS: https://github.com/owner/repo.git
		u, parseErr := url.Parse(remoteURL)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("parsing URL %q: %w", remoteURL, parseErr)
		}
		host = u.Hostname()
		path = strings.TrimPrefix(u.Path, "/")
	} else if strings.Contains(remoteURL, ":") {
		// SSH: git@github.com:owner/repo.git
		idx := strings.Index(remoteURL, ":")
		hostPart := remoteURL[:idx]
		// Strip user@ prefix (e.g., "git@github.com" -> "github.com")
		if at := strings.Index(hostPart, "@"); at >= 0 {
			host = hostPart[at+1:]
		} else {
			host = hostPart
		}
		path = remoteURL[idx+1:]
	} else {
		return "", "", "", fmt.Errorf("unrecognized URL format: %q", remoteURL)
	}

	// Strip .git suffix
	path = strings.TrimSuffix(path, ".git")
	// Strip trailing slash
	path = strings.TrimRight(path, "/")

	// Split into owner and repo. For nested groups like "group/subgroup/repo",
	// everything before the last segment is the owner.
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return "", "", "", fmt.Errorf("cannot determine owner/repo from path %q", path)
	}

	owner = path[:lastSlash]
	repo = path[lastSlash+1:]

	if owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("cannot determine owner/repo from path %q", path)
	}

	return host, owner, repo, nil
}
