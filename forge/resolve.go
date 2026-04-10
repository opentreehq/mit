package forge

import "fmt"

// ForForgeType returns the Forge implementation for the given type.
// The returned forge targets the default SaaS host (github.com / gitlab.com).
// Use ResolveForge to get a host-aware forge for self-hosted instances.
func ForForgeType(ft ForgeType) (Forge, error) {
	switch ft {
	case GitHub:
		return &GitHubForge{}, nil
	case GitLab:
		return &GitLabForge{}, nil
	default:
		return nil, fmt.Errorf("unknown forge type %q; supported: github, gitlab", ft)
	}
}

// ResolveForge determines the forge, owner, and repo for a resolved repo config.
// The returned Forge is configured with the correct host from the repo URL,
// so self-hosted instances (e.g., gitlab.example.com) work automatically.
func ResolveForge(name, repoURL, forgeStr string) (Forge, string, string, error) {
	if forgeStr == "" {
		return nil, "", "", fmt.Errorf("no forge configured for repo %q; set 'forge: github' or 'forge: gitlab' in workspace config or on the repo", name)
	}

	host, owner, repo, err := ParseHostOwnerRepo(repoURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("repo %q: %w", name, err)
	}

	ft := ForgeType(forgeStr)
	var f Forge
	switch ft {
	case GitHub:
		f = &GitHubForge{Host: host}
	case GitLab:
		f = &GitLabForge{Host: host}
	default:
		return nil, "", "", fmt.Errorf("repo %q: unknown forge type %q; supported: github, gitlab", name, ft)
	}

	return f, owner, repo, nil
}
