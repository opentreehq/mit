package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "List current branches across all repos",
	RunE:  runBranch,
}

var branchCommon bool

func init() {
	branchCmd.Flags().BoolVar(&branchCommon, "common", false, "show only branches that exist in all repos")
	rootCmd.AddCommand(branchCmd)
}

func runBranch(cmd *cobra.Command, args []string) error {
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

	type branchResult struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
	}

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
				branch, err := driver.CurrentBranch(ctx, repo.AbsPath)
				if err != nil {
					return nil, err
				}
				return &executor.Result{
					Success: true,
					Data:    branchResult{Repo: repo.Name, Branch: branch},
				}, nil
			},
		})
	}

	exec := executor.New(getParallelism(), true, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	var branches []branchResult
	branchCount := make(map[string]int)
	for _, r := range results {
		if r.Data != nil {
			if br, ok := r.Data.(branchResult); ok {
				branches = append(branches, br)
				branchCount[br.Branch]++
			}
		}
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("branch", branches)
		return output.New("json").Format(env)
	}

	if branchCommon {
		totalRepos := len(tasks)
		fmt.Println("Common branches (present in all repos):")
		found := false
		// Sort for consistent output
		branchNames := make([]string, 0, len(branchCount))
		for b := range branchCount {
			branchNames = append(branchNames, b)
		}
		sort.Strings(branchNames)
		for _, b := range branchNames {
			if branchCount[b] == totalRepos {
				fmt.Printf("  %s\n", b)
				found = true
			}
		}
		if !found {
			fmt.Println("  (none)")
		}
		return nil
	}

	headers := []string{"REPO", "BRANCH"}
	var rows [][]string
	for _, br := range branches {
		rows = append(rows, []string{br.Repo, br.Branch})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}
