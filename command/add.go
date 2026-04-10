package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabemeola/mit/config"
	"github.com/gabemeola/mit/vcs"
	"github.com/urfave/cli/v3"
)

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a new repo to the workspace",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Usage:   "repo name (default: derived from URL)",
				Local:   true,
			},
			&cli.StringFlag{
				Name:    "branch",
				Value:   "main",
				Usage:   "default branch",
				Local:   true,
			},
			&cli.StringFlag{
				Name:    "path",
				Usage:   "local path (default: repo name)",
				Local:   true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected 1 argument(s)")
			}
			url := cmd.Args().First()
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

			name := cmd.String("name")
			if name == "" {
				name = repoNameFromURL(url)
			}

			if _, exists := cfg.Repos[name]; exists {
				return fmt.Errorf("repo %q already exists in workspace", name)
			}

			repo := config.Repo{
				URL:    url,
				Branch: cmd.String("branch"),
			}
			if p := cmd.String("path"); p != "" {
				repo.Path = p
			}

			cfg.Repos[name] = repo

			if err := config.Save(root, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Added %s to %s\n", name, config.ConfigFileName)

			resolved := repo.Resolve(name, cfg.Workspace.Forge)
			absPath := filepath.Join(root, resolved.Path)
			if _, err := os.Stat(absPath); err != nil {
				if !isDryRun(cmd) {
					fmt.Printf("Cloning %s...\n", name)
					driver := vcs.NewGitDriver()
					if err := driver.Clone(context.Background(), url, absPath, resolved.Branch); err != nil {
						return fmt.Errorf("cloning: %w", err)
					}
					fmt.Printf("Cloned %s to %s\n", name, resolved.Path)
				}
			}

			return nil
		},
	}
}

func repoNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	return name
}
