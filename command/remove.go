package command

import (
	"context"
	"fmt"
	"os"

	"github.com/gabemeola/mit/config"
	"github.com/urfave/cli/v3"
)

func RemoveCommand() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"rm"},
		Usage:   "Remove a repo from mit.yaml",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d", cmd.Args().Len())
			}
			name := cmd.Args().First()
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

			if _, exists := cfg.Repos[name]; !exists {
				return fmt.Errorf("repo %q not found in workspace", name)
			}

			delete(cfg.Repos, name)

			if err := config.Save(root, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Removed %s from %s\n", name, config.ConfigFileName)
			fmt.Println("Note: the directory was not deleted. Remove it manually if needed.")
			return nil
		},
	}
}
