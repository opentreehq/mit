package command

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/gabemeola/mit/executor"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

// BranchCommand returns the branch subcommand.
func BranchCommand() *cli.Command {
	return &cli.Command{
		Name:  "branch",
		Usage: "List current branches across all repos",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "common",
				Usage: "show only branches that exist in all repos",
				Local: true,
			},
		},
		Action: branchAction,
	}
}

func branchAction(ctx context.Context, cmd *cli.Command) error {
	type branchResult struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
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

	exec := executor.New(getParallelism(cmd), true, os.Stdout)
	results := exec.Run(ctx, tasks)

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

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("branch", branches)
		return output.New("json").Format(env)
	}

	if cmd.Bool("common") {
		totalRepos := len(tasks)
		fmt.Println("Common branches (present in all repos):")
		found := false
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
