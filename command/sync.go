package command

import (
	"context"
	"fmt"
	"os"

	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

// SyncCommand returns the sync subcommand.
func SyncCommand() *cli.Command {
	return &cli.Command{
		Name:        "sync",
		Usage:       "Pull latest default branch for all repos",
		Description: "Checks out the default branch (from mit.yaml) and pulls latest for each repo.",
		Action:      syncAction,
	}
}

func syncAction(ctx context.Context, cmd *cli.Command) error {
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

				// Checkout default branch
				if err := driver.Checkout(ctx, repo.AbsPath, repo.Branch, false); err != nil {
					return nil, fmt.Errorf("checkout %s: %w", repo.Branch, err)
				}

				// Pull latest
				if err := driver.Pull(ctx, repo.AbsPath); err != nil {
					return nil, fmt.Errorf("pull: %w", err)
				}

				return &executor.Result{Success: true}, nil
			},
		})
	}

	if len(tasks) == 0 {
		fmt.Println("No repos to sync")
		return nil
	}

	if isDryRun(cmd) {
		for _, t := range tasks {
			dryRunMsg(cmd, "would sync %s", t.RepoName)
		}
		return nil
	}

	fmt.Printf("Syncing %d repos...\n", len(tasks))
	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("sync", results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return fmt.Errorf("%d repos failed to sync", errCount)
	}

	fmt.Printf("Successfully synced %d repos\n", len(tasks))
	return nil
}
