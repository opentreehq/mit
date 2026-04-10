package command

import (
	"context"
	"fmt"
	"os"

	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/workspace"
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
				MitDir: fmt.Sprintf("%s/.mit", ws.Root),
			}

			if _, err := os.Stat(fmt.Sprintf("%s/.mit/state.db", ws.Root)); err == nil {
				info.HasStateDB = true
			}
			if _, err := os.Stat(fmt.Sprintf("%s/.mit/memory", ws.Root)); err == nil {
				info.HasMemory = true
			}
			if _, err := os.Stat(fmt.Sprintf("%s/.mit/skills", ws.Root)); err == nil {
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
