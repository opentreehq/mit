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

// WorktreeCommand returns the worktree command with create, list, and remove subcommands.
func WorktreeCommand() *cli.Command {
	return &cli.Command{
		Name:        "worktree",
		Usage:       "Manage worktrees across repos (for AI agents)",
		Description: "Manage worktrees across repos (for AI agents).",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create worktrees across all repos",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "branch",
						Aliases: []string{"b"},
						Usage:   "branch name for worktrees",
						Local:   true,
					},
				},
				Action: worktreeCreateAction,
			},
			{
				Name:   "list",
				Usage:  "List active worktrees",
				Action: worktreeListAction,
			},
			{
				Name:   "remove",
				Usage:  "Remove worktrees across all repos",
				Action: worktreeRemoveAction,
			},
		},
	}
}

func worktreeCreateAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	name := cmd.Args().First()
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

	branch := cmd.String("branch")
	if branch == "" {
		branch = name
	}

	var tasks []executor.Task
	for _, repo := range repos {
		repo := repo
		if !repo.Exists || repo.Driver == nil {
			continue
		}
		tasks = append(tasks, executor.Task{
			RepoName: repo.Name,
			Fn: func(ctx context.Context) (*executor.Result, error) {
				wtPath, err := repo.Driver.WorktreeAdd(ctx, repo.AbsPath, name, branch)
				if err != nil {
					return nil, err
				}
				return &executor.Result{Success: true, Output: wtPath}, nil
			},
		})
	}

	if isDryRun(cmd) {
		for _, t := range tasks {
			dryRunMsg(cmd, "would create worktree %q in %s", name, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Creating worktree %q across %d repos...\n", name, len(tasks))
	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("worktree create", results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
	}
	fmt.Printf("Created worktree %q in %d repos\n", name, len(tasks)-errCount)
	return nil
}

func worktreeListAction(_ context.Context, cmd *cli.Command) error {
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

	for _, repo := range repos {
		if !repo.Exists || repo.Driver == nil {
			continue
		}
		wts, err := repo.Driver.WorktreeList(context.Background(), repo.AbsPath)
		if err != nil || len(wts) <= 1 {
			continue // Skip repos with only the main worktree
		}
		fmt.Printf("%s:\n", repo.Name)
		for _, wt := range wts {
			fmt.Printf("  %s -> %s (%s)\n", wt.Name, wt.Path, wt.Branch)
		}
	}
	return nil
}

func worktreeRemoveAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	name := cmd.Args().First()
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
		if !repo.Exists || repo.Driver == nil {
			continue
		}
		tasks = append(tasks, executor.Task{
			RepoName: repo.Name,
			Fn: func(ctx context.Context) (*executor.Result, error) {
				if err := repo.Driver.WorktreeRemove(ctx, repo.AbsPath, name); err != nil {
					return nil, err
				}
				return &executor.Result{Success: true}, nil
			},
		})
	}

	if isDryRun(cmd) {
		for _, t := range tasks {
			dryRunMsg(cmd, "would remove worktree %q from %s", name, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Removing worktree %q across %d repos...\n", name, len(tasks))
	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
	}
	fmt.Printf("Removed worktree %q from %d repos\n", name, len(tasks)-errCount)
	return nil
}
