package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opentreehq/mit/config"
	"github.com/urfave/cli/v3"
)

func InitCommand() *cli.Command {
	return &cli.Command{
		Name:        "init",
		Usage:       "Initialize a new mit workspace",
		Description: "Create a new mit.yaml configuration file in the current directory.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "name",
				Value: "",
				Usage: "workspace name (default: directory name)",
				Local: true,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			configPath := filepath.Join(dir, config.ConfigFileName)
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("%s already exists in this directory", config.ConfigFileName)
			}

			name := cmd.String("name")
			if name == "" {
				name = filepath.Base(dir)
			}

			cfg := &config.Config{
				Version: "1",
				Workspace: config.WorkspaceConfig{
					Name: name,
				},
				Repos: map[string]config.Repo{},
			}

			if err := config.Save(dir, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Created %s for workspace %q\n", config.ConfigFileName, name)
			fmt.Printf("Add repos with: %s add <url>\n", cmd.Root().Name)
			return nil
		},
	}
}
