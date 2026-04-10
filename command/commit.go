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

// CommitCommand returns the commit subcommand.
func CommitCommand() *cli.Command {
	return &cli.Command{
		Name:  "commit",
		Usage: "Commit in all dirty repos",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "message",
				Aliases: []string{"m"},
				Usage:   "commit message (required)",
				Local:   true,
			},
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "stage all modified files before committing",
				Local:   true,
			},
		},
		Action: commitAction,
	}
}

func commitAction(ctx context.Context, cmd *cli.Command) error {
	msg := cmd.String("message")
	if msg == "" {
		return fmt.Errorf(`required flag "message" not set`)
	}
	all := cmd.Bool("all")

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

				status, err := driver.Status(ctx, repo.AbsPath)
				if err != nil {
					return nil, err
				}
				if !status.Dirty {
					return &executor.Result{Success: true, Output: "clean, skipped"}, nil
				}

				if err := driver.Commit(ctx, repo.AbsPath, msg, all); err != nil {
					return nil, err
				}
				return &executor.Result{Success: true, Output: "committed"}, nil
			},
		})
	}

	if isDryRun(cmd) {
		for _, t := range tasks {
			dryRunMsg(cmd, "would commit in %s", t.RepoName)
		}
		return nil
	}

	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
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
