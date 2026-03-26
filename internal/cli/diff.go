package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show diffs across all repos",
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

type diffResult struct {
	Repo string `json:"repo"`
	Diff string `json:"diff"`
}

func runDiff(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(flagRepos, flagExclude)
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

	exec := executor.New(getParallelism(), true, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	var diffs []diffResult
	for _, r := range results {
		if r.Data != nil {
			if dr, ok := r.Data.(diffResult); ok && dr.Diff != "" {
				diffs = append(diffs, dr)
			}
		}
	}

	if getOutputFormat() == "json" {
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
