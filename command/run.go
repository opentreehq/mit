package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

func RunCommand() *cli.Command {
	return &cli.Command{
		Name:        "run",
		Usage:       "Execute a shell command in each repo directory",
		Description: "Run an arbitrary command in each repo directory. Use -- to separate mit flags from the command.",
		Action:      runAction,
	}
}

func runAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() < 1 {
		return fmt.Errorf("requires at least 1 arg(s), only received %d", cmd.Args().Len())
	}

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

	args := cmd.Args().Slice()
	shellCmd := strings.Join(args, " ")

	var tasks []executor.Task
	for _, repo := range repos {
		repo := repo
		if !repo.Exists {
			continue
		}

		if isDryRun(cmd) {
			dryRunMsg(cmd, "would run %q in %s", shellCmd, repo.Name)
			continue
		}

		tasks = append(tasks, executor.Task{
			RepoName: repo.Name,
			Fn: func(ctx context.Context) (*executor.Result, error) {
				c := exec.CommandContext(ctx, "sh", "-c", shellCmd)
				c.Dir = repo.AbsPath
				out, err := c.CombinedOutput()
				outStr := strings.TrimSpace(string(out))
				if err != nil {
					return &executor.Result{
						Success: false,
						Output:  outStr,
						Error:   err.Error(),
					}, nil
				}
				return &executor.Result{
					Success: true,
					Output:  outStr,
				}, nil
			},
		})
	}

	if isDryRun(cmd) || len(tasks) == 0 {
		return nil
	}

	exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
	results := exec.Run(ctx, tasks)

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("run", results)
		env.Errors = executor.ErrorSummary(results)
		env.Success = executor.CountErrors(results) == 0
		return output.New("json").Format(env)
	}

	for _, r := range results {
		if r.Output != "" {
			fmt.Printf("=== %s ===\n%s\n\n", r.RepoName, r.Output)
		}
		if !r.Success {
			fmt.Fprintf(os.Stderr, "=== %s === ERROR: %s\n", r.RepoName, r.Error)
		}
	}

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		return fmt.Errorf("%d repos had errors", errCount)
	}
	return nil
}
