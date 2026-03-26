package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/vcs"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var grepCmd = &cobra.Command{
	Use:   "grep <pattern>",
	Short: "Search across all repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runGrep,
}

func init() {
	rootCmd.AddCommand(grepCmd)
}

type grepResultItem struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func runGrep(cmd *cobra.Command, args []string) error {
	pattern := args[0]
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
				driver, err := vcs.Detect(repo.AbsPath)
				if err != nil {
					return nil, err
				}
				results, err := driver.Grep(ctx, repo.AbsPath, pattern)
				if err != nil {
					return nil, err
				}
				var items []grepResultItem
				for _, r := range results {
					items = append(items, grepResultItem{
						Repo:    repo.Name,
						File:    r.File,
						Line:    r.Line,
						Content: r.Content,
					})
				}
				return &executor.Result{Success: true, Data: items}, nil
			},
		})
	}

	exec := executor.New(getParallelism(), true, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	var allItems []grepResultItem
	for _, r := range results {
		if r.Data != nil {
			if items, ok := r.Data.([]grepResultItem); ok {
				allItems = append(allItems, items...)
			}
		}
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("grep", allItems)
		env.Summary = map[string]int{"matches": len(allItems)}
		return output.New("json").Format(env)
	}

	repoColor := color.New(color.FgCyan, color.Bold)
	fileColor := color.New(color.FgMagenta)
	lineColor := color.New(color.FgGreen)

	for _, item := range allItems {
		repoColor.Fprintf(os.Stdout, "%s", item.Repo)
		fmt.Fprint(os.Stdout, ":")
		fileColor.Fprintf(os.Stdout, "%s", item.File)
		fmt.Fprint(os.Stdout, ":")
		lineColor.Fprintf(os.Stdout, "%d", item.Line)
		fmt.Fprintf(os.Stdout, ":%s\n", item.Content)
	}

	if len(allItems) == 0 {
		fmt.Println("No matches found")
	}
	return nil
}
