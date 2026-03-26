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

var switchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Switch branches across all repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwitch,
}

var switchCreate bool

func init() {
	switchCmd.Flags().BoolVarP(&switchCreate, "create", "c", false, "create the branch if it doesn't exist")
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) error {
	branch := args[0]
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
				if err := driver.Checkout(ctx, repo.AbsPath, branch, switchCreate); err != nil {
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

	if flagDryRun {
		for _, t := range tasks {
			action := "switch"
			if switchCreate {
				action = "create and switch"
			}
			dryRunMsg("would %s %s to %s", action, t.RepoName, branch)
		}
		return nil
	}

	action := "Switching"
	if switchCreate {
		action = "Creating and switching"
	}
	fmt.Printf("%s %d repos to %s...\n", action, len(tasks), branch)

	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
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
