package cli

import (
	"fmt"
	"os"

	"github.com/gabemeola/mit/internal/config"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a repo from mit.yaml",
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
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
}
