package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/config"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

func DoctorCommand() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Validate workspace health",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			ws, err := workspace.Load(dir)
			if err != nil {
				return err
			}

			var checks []doctorCheck

			for _, tool := range []string{"git", "sl"} {
				_, err := exec.LookPath(tool)
				status := "ok"
				detail := "installed"
				if err != nil {
					status = "warning"
					detail = "not found in PATH"
				}
				checks = append(checks, doctorCheck{
					Name:   fmt.Sprintf("vcs/%s", tool),
					Status: status,
					Detail: detail,
				})
			}

			sel := workspace.NewSelector(cmd.String("repos"), cmd.String("exclude"))
			repos := ws.FilterRepos(sel)

			for _, repo := range repos {
				status := "ok"
				detail := "exists"
				if !repo.Exists {
					status = "error"
					detail = "directory not found"
				} else if repo.Driver == nil {
					status = "warning"
					detail = "no VCS detected"
				} else {
					detail = fmt.Sprintf("%s repo", repo.Driver.Name())
				}
				checks = append(checks, doctorCheck{
					Name:   fmt.Sprintf("repo/%s", repo.Name),
					Status: status,
					Detail: detail,
				})
			}

			forgesNeeded := map[string]bool{}
			for _, repo := range repos {
				if repo.Forge != "" {
					forgesNeeded[repo.Forge] = true
				}
			}
			for forgeName := range forgesNeeded {
				tool := "gh"
				if forgeName == "gitlab" {
					tool = "glab"
				}
				_, lookErr := exec.LookPath(tool)
				status := "ok"
				detail := "installed"
				if lookErr != nil {
					status = "warning"
					detail = "not found in PATH"
				}
				checks = append(checks, doctorCheck{
					Name:   fmt.Sprintf("forge/%s", tool),
					Status: status,
					Detail: detail,
				})
			}

			if _, err := os.Stat(filepath.Join(ws.Root, config.DataDir)); err != nil {
				checks = append(checks, doctorCheck{
					Name:   "state/db",
					Status: "warning",
					Detail: config.DataDir + " directory not found (run index or task to create)",
				})
			}

			if getOutputFormat(cmd) == "json" {
				env := output.NewEnvelope("doctor", checks)
				hasErrors := false
				for _, c := range checks {
					if c.Status == "error" {
						hasErrors = true
					}
				}
				env.Success = !hasErrors
				return output.New("json").Format(env)
			}

			ok := color.New(color.FgGreen)
			warn := color.New(color.FgYellow)
			errC := color.New(color.FgRed)

			for _, c := range checks {
				var icon string
				switch c.Status {
				case "ok":
					icon = ok.Sprint("[ok]")
				case "warning":
					icon = warn.Sprint("[warn]")
				case "error":
					icon = errC.Sprint("[err]")
				}
				fmt.Printf("  %s %-30s %s\n", icon, c.Name, c.Detail)
			}

			return nil
		},
	}
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, error
	Detail string `json:"detail,omitempty"`
}
