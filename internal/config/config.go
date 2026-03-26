package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = "mit.yaml"

// Load reads and parses mit.yaml from the given directory.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ConfigFileName)
	return LoadFile(path)
}

// LoadFile reads and parses a mit.yaml from the given path.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return Parse(data)
}

// Parse parses mit.yaml content from bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to mit.yaml in the given directory.
func Save(dir string, cfg *Config) error {
	path := filepath.Join(dir, ConfigFileName)
	return SaveFile(path, cfg)
}

// SaveFile writes the config to the given path.
func SaveFile(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// FindRoot walks up from dir looking for mit.yaml, returns the directory containing it.
func FindRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ConfigFileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in any parent directory", ConfigFileName)
		}
		dir = parent
	}
}

func validate(cfg *Config) error {
	if cfg.Version == "" {
		cfg.Version = "1"
	}
	if cfg.Workspace.Name == "" {
		return fmt.Errorf("workspace.name is required")
	}
	if len(cfg.Repos) == 0 {
		return fmt.Errorf("at least one repo is required")
	}
	for name, repo := range cfg.Repos {
		if repo.URL == "" {
			return fmt.Errorf("repo %q: url is required", name)
		}
	}
	return nil
}
