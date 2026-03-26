package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Execute a shell command in each repo directory",
	Long:  "Run an arbitrary command in each repo directory. Use -- to separate mit flags from the command.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
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

	shellCmd := strings.Join(args, " ")

	var tasks []executor.Task
	for _, repo := range repos {
		repo := repo
		if !repo.Exists {
			continue
		}

		if flagDryRun {
			dryRunMsg("would run %q in %s", shellCmd, repo.Name)
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

	if flagDryRun || len(tasks) == 0 {
		return nil
	}

	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
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
