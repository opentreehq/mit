package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/vcs"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Interleaved commit log across repos",
	RunE:  runLog,
}

var logLimit int

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 10, "number of commits per repo")
	rootCmd.AddCommand(logCmd)
}

type logEntry struct {
	Repo    string `json:"repo"`
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

func runLog(cmd *cobra.Command, args []string) error {
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
				commits, err := driver.Log(ctx, repo.AbsPath, logLimit)
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

	exec := executor.New(getParallelism(), true, os.Stdout)
	results := exec.Run(context.Background(), tasks)

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

	if getOutputFormat() == "json" {
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
