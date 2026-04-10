package command

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/executor"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

func DiffCommand() *cli.Command {
	return &cli.Command{
		Name:   "diff",
		Usage:  "Show diffs across all repos",
		Action: diffAction,
	}
}

type diffResult struct {
	Repo string `json:"repo"`
	Diff string `json:"diff"`
}

func diffAction(ctx context.Context, cmd *cli.Command) error {
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
		if !repo.Exists {
			continue
		}
		tasks = append(tasks, executor.Task{
			RepoName: repo.Name,
			Fn: func(ctx context.Context) (*executor.Result, error) {
				driver, err := ws.EnsureDriver(ctx, &repo, "git")
				if err != nil {
					return nil, err
				}
				diff, err := driver.Diff(ctx, repo.AbsPath)
				if err != nil {
					return nil, err
				}
				return &executor.Result{
					Success: true,
					Data:    diffResult{Repo: repo.Name, Diff: diff},
				}, nil
			},
		})
	}

	exec := executor.New(getParallelism(cmd), true, os.Stdout)
	results := exec.Run(ctx, tasks)

	var diffs []diffResult
	for _, r := range results {
		if r.Data != nil {
			if dr, ok := r.Data.(diffResult); ok && dr.Diff != "" {
				diffs = append(diffs, dr)
			}
		}
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("diff", diffs)
		return output.New("json").Format(env)
	}

	if len(diffs) == 0 {
		fmt.Println("No diffs found")
		return nil
	}

	header := color.New(color.FgCyan, color.Bold)
	for _, d := range diffs {
		header.Fprintf(os.Stdout, "=== %s ===\n", d.Repo)
		fmt.Println(d.Diff)
	}
	return nil
}
