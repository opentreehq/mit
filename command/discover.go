package command

import (
	"context"
	"os"
	"path/filepath"

	"github.com/opentreehq/mit/config"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

func DiscoverCommand() *cli.Command {
	return &cli.Command{
		Name:  "discover",
		Usage: "Output full workspace topology as JSON (for agents)",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_ = getOutputFormat(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			ws, err := workspace.Load(dir)
			if err != nil {
				return err
			}

			info := discoveryInfo{
				Workspace: discoveryWorkspace{
					Name:        ws.Config.Workspace.Name,
					Description: ws.Config.Workspace.Description,
					Root:        ws.Root,
				},
				MitDir: filepath.Join(ws.Root, config.DataDir),
			}

			dataDir := filepath.Join(ws.Root, config.DataDir)
			if _, err := os.Stat(filepath.Join(dataDir, "state.db")); err == nil {
				info.HasStateDB = true
			}
			if _, err := os.Stat(filepath.Join(dataDir, "memory")); err == nil {
				info.HasMemory = true
			}
			if _, err := os.Stat(filepath.Join(dataDir, "skills")); err == nil {
				info.HasSkills = true
			}

			for _, repo := range ws.Repos {
				vcsName := ""
				if repo.Driver != nil {
					vcsName = repo.Driver.Name()
				}
				info.Repos = append(info.Repos, discoveryRepo{
					Name:    repo.Name,
					URL:     repo.URL,
					Path:    repo.Path,
					AbsPath: repo.AbsPath,
					Branch:  repo.Branch,
					VCS:     vcsName,
					Exists:  repo.Exists,
				})
			}

			env := output.NewEnvelope("discover", info)
			return output.New("json").Format(env)
		},
	}
}

type discoveryInfo struct {
	Workspace  discoveryWorkspace `json:"workspace"`
	Repos      []discoveryRepo    `json:"repos"`
	MitDir     string             `json:"mit_dir"`
	HasStateDB bool               `json:"has_state_db"`
	HasMemory  bool               `json:"has_memory"`
	HasSkills  bool               `json:"has_skills"`
}

type discoveryWorkspace struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Root        string `json:"root"`
}

type discoveryRepo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Path    string `json:"path"`
	AbsPath string `json:"abs_path"`
	Branch  string `json:"branch"`
	VCS     string `json:"vcs,omitempty"`
	Exists  bool   `json:"exists"`
}
