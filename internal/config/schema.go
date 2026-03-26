package config

// Config represents the top-level mit.yaml configuration.
type Config struct {
	Version   string            `yaml:"version"`
	Workspace WorkspaceConfig   `yaml:"workspace"`
	Repos     map[string]Repo   `yaml:"repos"`
	Index     IndexConfig       `yaml:"index,omitempty"`
}

// WorkspaceConfig holds workspace-level metadata.
type WorkspaceConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// IndexConfig holds configuration for the semantic index.
type IndexConfig struct {
	Ignore []string     `yaml:"ignore,omitempty"`
	Model  *ModelConfig `yaml:"model,omitempty"`
}

// ModelConfig specifies a custom embedding model.
type ModelConfig struct {
	URL string `yaml:"url"`
}

// DefaultIndexIgnore contains names (dirs and files) always skipped during indexing.
var DefaultIndexIgnore = []string{
	// directories
	".git", ".sl", "node_modules", "vendor", ".next",
	"dist", "build", "__pycache__", ".cache", "target",
	".mit", ".mit-worktrees",
	// lock files (no semantic value)
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"go.sum", "Gemfile.lock", "poetry.lock", "composer.lock",
	"Cargo.lock", "Pipfile.lock",
	// secrets / environment files
	".env",
}

// IndexIgnoreSet returns a set of all directories to ignore during indexing,
// combining defaults with user-configured ignores.
func (c *Config) IndexIgnoreSet() map[string]bool {
	set := make(map[string]bool, len(DefaultIndexIgnore)+len(c.Index.Ignore))
	for _, d := range DefaultIndexIgnore {
		set[d] = true
	}
	for _, d := range c.Index.Ignore {
		set[d] = true
	}
	return set
}

// Repo represents a single repository declaration in mit.yaml.
type Repo struct {
	URL    string `yaml:"url"`
	Path   string `yaml:"path,omitempty"`
	Branch string `yaml:"branch,omitempty"`
}

// ResolvedRepo is a Repo with all defaults applied.
type ResolvedRepo struct {
	Name   string
	URL    string
	Path   string
	Branch string
}

// Resolve applies defaults to the repo configuration.
// name is the key from the repos map.
func (r Repo) Resolve(name string) ResolvedRepo {
	resolved := ResolvedRepo{
		Name:   name,
		URL:    r.URL,
		Path:   r.Path,
		Branch: r.Branch,
	}
	if resolved.Path == "" {
		resolved.Path = name
	}
	if resolved.Branch == "" {
		resolved.Branch = "main"
	}
	return resolved
}

// ResolveAll returns all repos with defaults applied.
func (c *Config) ResolveAll() []ResolvedRepo {
	repos := make([]ResolvedRepo, 0, len(c.Repos))
	for name, repo := range c.Repos {
		repos = append(repos, repo.Resolve(name))
	}
	return repos
}
