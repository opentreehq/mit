package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/vcs"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a new repo to the workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var addName string
var addBranch string
var addPath string

func init() {
	addCmd.Flags().StringVar(&addName, "name", "", "repo name (default: derived from URL)")
	addCmd.Flags().StringVar(&addBranch, "branch", "main", "default branch")
	addCmd.Flags().StringVar(&addPath, "path", "", "local path (default: repo name)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	url := args[0]
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	root, err := config.FindRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	// Derive name from URL if not provided
	name := addName
	if name == "" {
		name = repoNameFromURL(url)
	}

	if _, exists := cfg.Repos[name]; exists {
		return fmt.Errorf("repo %q already exists in workspace", name)
	}

	repo := config.Repo{
		URL:    url,
		Branch: addBranch,
	}
	if addPath != "" {
		repo.Path = addPath
	}

	cfg.Repos[name] = repo

	if err := config.Save(root, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Added %s to %s\n", name, config.ConfigFileName)

	// Clone if not already present
	resolved := repo.Resolve(name)
	absPath := filepath.Join(root, resolved.Path)
	if _, err := os.Stat(absPath); err != nil {
		if !flagDryRun {
			fmt.Printf("Cloning %s...\n", name)
			driver, _ := vcs.DriverByName(cloneVCS)
			if driver == nil {
				driver = vcs.NewGitDriver()
			}
			if err := driver.Clone(context.Background(), url, absPath, resolved.Branch); err != nil {
				return fmt.Errorf("cloning: %w", err)
			}
			fmt.Printf("Cloned %s to %s\n", name, resolved.Path)
		}
	}

	return nil
}

func repoNameFromURL(url string) string {
	// Handle git@host:org/repo.git and https://host/org/repo.git
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	return name
}
