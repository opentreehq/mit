package command

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/vcs"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

// StatusCommand returns the status subcommand.
func StatusCommand() *cli.Command {
	return &cli.Command{
		Name:   "status",
		Usage:  "Show aggregated status of all repos",
		Action: statusAction,
	}
}

type statusResult struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	VCS    string `json:"vcs"`
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

func statusAction(ctx context.Context, cmd *cli.Command) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(cmd.String("repos"), cmd.String("exclude"))
	repos := ws.FilterRepos(sel)

	var tasks []executor.Task
	for _, repo := range repos {
		repo := repo
		tasks = append(tasks, executor.Task{
			RepoName: repo.Name,
			Fn: func(ctx context.Context) (*executor.Result, error) {
				if !repo.Exists {
					return &executor.Result{
						Success: true,
						Data: statusResult{
							Repo:   repo.Name,
							Path:   repo.Path,
							Exists: false,
						},
					}, nil
				}

				driver, err := vcs.Detect(repo.AbsPath)
				if err != nil {
					return nil, err
				}

				st, err := driver.Status(ctx, repo.AbsPath)
				if err != nil {
					return nil, err
				}

				return &executor.Result{
					Success: true,
					Data: statusResult{
						Repo:   repo.Name,
						Path:   repo.Path,
						Branch: st.Branch,
						Dirty:  st.Dirty,
						Ahead:  st.Ahead,
						Behind: st.Behind,
						VCS:    driver.Name(),
						Exists: true,
					},
				}, nil
			},
		})
	}

	exec := executor.New(getParallelism(cmd), true, os.Stdout)
	results := exec.Run(ctx, tasks)

	// Collect status data
	var statuses []statusResult
	for _, r := range results {
		if r.Data != nil {
			if sr, ok := r.Data.(statusResult); ok {
				statuses = append(statuses, sr)
			}
		} else if !r.Success {
			statuses = append(statuses, statusResult{
				Repo:  r.RepoName,
				Error: r.Error,
			})
		}
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("status", statuses)
		dirtyCount := 0
		for _, s := range statuses {
			if s.Dirty {
				dirtyCount++
			}
		}
		env.Summary = map[string]int{
			"total": len(statuses),
			"dirty": dirtyCount,
		}
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	// Table output
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	gray := color.New(color.FgHiBlack)

	headers := []string{"REPO", "BRANCH", "STATUS", "AHEAD", "BEHIND", "VCS"}
	var rows [][]string

	for _, s := range statuses {
		if s.Error != "" {
			rows = append(rows, []string{s.Repo, "", red.Sprint("error: " + s.Error), "", "", ""})
			continue
		}
		if !s.Exists {
			rows = append(rows, []string{s.Repo, "", gray.Sprint("not cloned"), "", "", ""})
			continue
		}

		statusStr := green.Sprint("clean")
		if s.Dirty {
			statusStr = yellow.Sprint("dirty")
		}

		aheadStr := fmt.Sprintf("%d", s.Ahead)
		behindStr := fmt.Sprintf("%d", s.Behind)
		if s.Ahead > 0 {
			aheadStr = green.Sprintf("+%d", s.Ahead)
		}
		if s.Behind > 0 {
			behindStr = red.Sprintf("-%d", s.Behind)
		}

		rows = append(rows, []string{s.Repo, s.Branch, statusStr, aheadStr, behindStr, s.VCS})
	}

	output.PrintTable(os.Stdout, headers, rows)
	return nil
}
