package command

import (
	"context"
	"fmt"
	"os"

	"github.com/gabemeola/mit/executor"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

// PullCommand returns the pull subcommand.
func PullCommand() *cli.Command {
	return &cli.Command{
		Name:   "pull",
		Usage:  "Pull current branch for all repos",
		Action: pullAction,
	}
}

// PushCommand returns the push subcommand.
func PushCommand() *cli.Command {
	return &cli.Command{
		Name:   "push",
		Usage:  "Push repos with local commits ahead of remote",
		Action: pushAction,
	}
}

// FetchCommand returns the fetch subcommand.
func FetchCommand() *cli.Command {
	return &cli.Command{
		Name:   "fetch",
		Usage:  "Fetch all remotes in parallel",
		Action: fetchAction,
	}
}

func pullAction(ctx context.Context, cmd *cli.Command) error {
	return runVCSBulkOp(ctx, cmd, "pull", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Pull(ctx, repo.AbsPath)
	})
}

func pushAction(ctx context.Context, cmd *cli.Command) error {
	return runVCSBulkOp(ctx, cmd, "push", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Push(ctx, repo.AbsPath)
	})
}

func fetchAction(ctx context.Context, cmd *cli.Command) error {
	return runVCSBulkOp(ctx, cmd, "fetch", func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error {
		driver, err := ws.EnsureDriver(ctx, &repo, "git")
		if err != nil {
			return err
		}
		return driver.Fetch(ctx, repo.AbsPath)
	})
}

// runVCSBulkOp is a helper that runs a VCS operation across all repos.
func runVCSBulkOp(ctx context.Context, cmd *cli.Command, opName string, fn func(ctx context.Context, repo workspace.RepoInfo, ws *workspace.Workspace) error) error {
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

	if isDryRun(cmd) {
		for _, t := range tasks {
			dryRunMsg(cmd, "would %s %s", opName, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Running %s on %d repos...\n", opName, len(tasks))
	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
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
