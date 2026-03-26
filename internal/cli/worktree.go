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

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage worktrees across repos (for AI agents)",
}

var worktreeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create worktrees across all repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorktreeCreate,
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active worktrees",
	RunE:  runWorktreeList,
}

var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove worktrees across all repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorktreeRemove,
}

var worktreeBranch string

func init() {
	worktreeCreateCmd.Flags().StringVarP(&worktreeBranch, "branch", "b", "", "branch name for worktrees")
	worktreeCmd.AddCommand(worktreeCreateCmd)
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeRemoveCmd)
	rootCmd.AddCommand(worktreeCmd)
}

func runWorktreeCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
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

	branch := worktreeBranch
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

	if flagDryRun {
		for _, t := range tasks {
			dryRunMsg("would create worktree %q in %s", name, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Creating worktree %q across %d repos...\n", name, len(tasks))
	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
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

func runWorktreeList(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	type wtInfo struct {
		Repo      string `json:"repo"`
		Worktrees []struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Branch string `json:"branch"`
		} `json:"worktrees"`
	}

	sel := workspace.NewSelector(flagRepos, flagExclude)
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

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
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

	if flagDryRun {
		for _, t := range tasks {
			dryRunMsg("would remove worktree %q from %s", name, t.RepoName)
		}
		return nil
	}

	fmt.Printf("Removing worktree %q across %d repos...\n", name, len(tasks))
	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	errCount := executor.CountErrors(results)
	if errCount > 0 {
		for _, e := range executor.ErrorSummary(results) {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
	}
	fmt.Printf("Removed worktree %q from %d repos\n", name, len(tasks)-errCount)
	return nil
}
