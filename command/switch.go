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

// SwitchCommand returns the switch subcommand.
func SwitchCommand() *cli.Command {
	return &cli.Command{
		Name:  "switch",
		Usage: "Switch branches across all repos",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "create",
				Aliases: []string{"c"},
				Usage:   "create the branch if it doesn't exist",
				Local:   true,
			},
		},
		Action: switchAction,
	}
}

func switchAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("accepts 1 arg(s), received %d", cmd.Args().Len())
	}
	branch := cmd.Args().First()
	create := cmd.Bool("create")

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
				if err := driver.Checkout(ctx, repo.AbsPath, branch, create); err != nil {
					return nil, err
				}
				return &executor.Result{Success: true}, nil
			},
		})
	}

	if len(tasks) == 0 {
		fmt.Println("No repos to switch")
		return nil
	}

	if isDryRun(cmd) {
		for _, t := range tasks {
			action := "switch"
			if create {
				action = "create and switch"
			}
			dryRunMsg(cmd, "would %s %s to %s", action, t.RepoName, branch)
		}
		return nil
	}

	action := "Switching"
	if create {
		action = "Creating and switching"
	}
	fmt.Printf("%s %d repos to %s...\n", action, len(tasks), branch)

	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("switch", results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return fmt.Errorf("%d repos failed to switch", errCount)
	}

	fmt.Printf("Successfully switched %d repos to %s\n", len(tasks), branch)
	return nil
}
