package command

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/vcs"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

func GrepCommand() *cli.Command {
	return &cli.Command{
		Name:   "grep",
		Usage:  "Search across all repos",
		Action: grepAction,
	}
}

type grepResultItem struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func grepAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	pattern := cmd.Args().First()

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

	exec := executor.New(getParallelism(cmd), true, os.Stdout)
	results := exec.Run(ctx, tasks)

	var allItems []grepResultItem
	for _, r := range results {
		if r.Data != nil {
			if items, ok := r.Data.([]grepResultItem); ok {
				allItems = append(allItems, items...)
			}
		}
	}

	if getOutputFormat(cmd) == "json" {
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
