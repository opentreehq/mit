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

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull current branch for all repos",
	RunE:  runPull,
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push repos with local commits ahead of remote",
	RunE:  runPush,
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch all remotes in parallel",
	RunE:  runFetch,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(fetchCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	return runVCSBulkOp("pull", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Pull(ctx, repo.AbsPath)
	})
}

func runPush(cmd *cobra.Command, args []string) error {
	return runVCSBulkOp("push", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Push(ctx, repo.AbsPath)
	})
}

func runFetch(cmd *cobra.Command, args []string) error {
	return runVCSBulkOp("fetch", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Fetch(ctx, repo.AbsPath)
	})
}

// runVCSBulkOp is a helper that runs a VCS operation across all repos.
func runVCSBulkOp(opName string, fn func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error) error {
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
				if err := fn(ctx, repo, ws); err != nil {
					return nil, err
				}
				return &executor.Result{Success: true}, nil
			},
		})
	}

	if len(tasks) == 0 {
		fmt.Printf("No repos to %s\n", opName)
		return nil
	}

	if flagDryRun {
		for _, t := range tasks {
			dryRunMsg("would %s %s", opName, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Running %s on %d repos...\n", opName, len(tasks))
	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
		env := output.NewEnvelope(opName, results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return fmt.Errorf("%d repos failed", errCount)
	}

	fmt.Printf("Successfully ran %s on %d repos\n", opName, len(tasks))
	return nil
}
