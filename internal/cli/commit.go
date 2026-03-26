package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit in all dirty repos",
	RunE:  runCommit,
}

var commitMessage string
var commitAll bool

func init() {
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "commit message (required)")
	commitCmd.Flags().BoolVarP(&commitAll, "all", "a", false, "stage all modified files before committing")
	commitCmd.MarkFlagRequired("message")
	rootCmd.AddCommand(commitCmd)
}

func runCommit(cmd *cobra.Command, args []string) error {
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

				// Check if repo is dirty
				status, err := driver.Status(ctx, repo.AbsPath)
				if err != nil {
					return nil, err
				}
				if !status.Dirty {
					return &executor.Result{Success: true, Output: "clean, skipped"}, nil
				}

				if err := driver.Commit(ctx, repo.AbsPath, commitMessage, commitAll); err != nil {
					return nil, err
				}
				return &executor.Result{Success: true, Output: "committed"}, nil
			},
		})
	}

	if flagDryRun {
		for _, t := range tasks {
			dryRunMsg("would commit in %s", t.RepoName)
		}
		return nil
	}

	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("commit", results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	committed := 0
	for _, r := range results {
		if r.Success && r.Output == "committed" {
			committed++
		}
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
	}

	fmt.Printf("Committed in %d repos\n", committed)
	return nil
}
