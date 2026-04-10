package command

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/fatih/color"
	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/vcs"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

func LogCommand() *cli.Command {
	return &cli.Command{
		Name:  "log",
		Usage: "Interleaved commit log across repos",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"n"},
				Value:   10,
				Usage:   "number of commits per repo",
				Local:   true,
			},
		},
		Action: logAction,
	}
}

type logEntry struct {
	Repo    string `json:"repo"`
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

func logAction(ctx context.Context, cmd *cli.Command) error {
	limit := int(cmd.Int("limit"))

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
				commits, err := driver.Log(ctx, repo.AbsPath, limit)
				if err != nil {
					return nil, err
				}
				var entries []logEntry
				for _, c := range commits {
					entries = append(entries, logEntry{
						Repo:    repo.Name,
						Hash:    c.Hash,
						Author:  c.Author,
						Date:    c.Date,
						Message: c.Message,
					})
				}
				return &executor.Result{Success: true, Data: entries}, nil
			},
		})
	}

	exec := executor.New(getParallelism(cmd), true, os.Stdout)
	results := exec.Run(ctx, tasks)

	var allEntries []logEntry
	for _, r := range results {
		if r.Data != nil {
			if entries, ok := r.Data.([]logEntry); ok {
				allEntries = append(allEntries, entries...)
			}
		}
	}

	// Sort by date descending
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Date > allEntries[j].Date
	})

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("log", allEntries)
		return output.New("json").Format(env)
	}

	repoColor := color.New(color.FgCyan)
	hashColor := color.New(color.FgYellow)
	for _, e := range allEntries {
		repoColor.Fprintf(os.Stdout, "%-20s", e.Repo)
		hashColor.Fprintf(os.Stdout, " %s", e.Hash[:8])
		fmt.Fprintf(os.Stdout, " %s %s - %s\n", e.Date[:10], e.Author, e.Message)
	}
	return nil
}
