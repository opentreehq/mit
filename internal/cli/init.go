package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabemeola/mit/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new mit workspace",
	Long:  "Create a new mit.yaml configuration file in the current directory.",
	RunE:  runInit,
}

var initName string

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "workspace name (default: directory name)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Check if mit.yaml already exists
	configPath := filepath.Join(dir, config.ConfigFileName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists in this directory", config.ConfigFileName)
	}

	name := initName
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

	// If there's no repos yet, add a placeholder comment via raw yaml
	if err := config.Save(dir, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Created %s for workspace %q\n", config.ConfigFileName, name)
	fmt.Println("Add repos with: mit add <url>")
	return nil
}
