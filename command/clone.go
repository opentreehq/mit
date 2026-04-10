package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opentreehq/mit/config"
	"github.com/opentreehq/mit/executor"
	"github.com/opentreehq/mit/output"
	"github.com/opentreehq/mit/vcs"
	"github.com/opentreehq/mit/workspace"
	"github.com/urfave/cli/v3"
)

func CloneCommand() *cli.Command {
	return &cli.Command{
		Name:  "clone",
		Usage: "Clone all repos defined in mit.yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "vcs",
				Value:   "git",
				Usage:   "VCS driver to use for cloning: git or sl",
				Local:   true,
			},
			&cli.IntFlag{
				Name:    "timeout",
				Value:   300,
				Usage:   "timeout per repo in seconds",
				Local:   true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg, err := config.Load(dir)
			if err != nil {
				return err
			}

			driver, err := vcs.DriverByName(cmd.String("vcs"))
			if err != nil {
				return err
			}

			sel := workspace.NewSelector(cmd.String("repos"), cmd.String("exclude"))
			repos := cfg.ResolveAll()

			cloneTimeout := cmd.Int("timeout")

			var tasks []executor.Task
			for _, repo := range repos {
				repo := repo
				if !sel.Matches(repo.Name) {
					continue
				}

				absPath := filepath.Join(dir, repo.Path)
				if info, err := os.Stat(absPath); err == nil {
					if info.IsDir() && isDirEmpty(absPath) {
						os.Remove(absPath)
					} else {
						if !isQuiet(cmd) {
							fmt.Printf("  %s already exists, skipping\n", repo.Name)
						}
						continue
					}
				}

				if isDryRun(cmd) {
					dryRunMsg(cmd, "would clone %s to %s", repo.URL, repo.Path)
					continue
				}

				tasks = append(tasks, executor.Task{
					RepoName: repo.Name,
					Fn: func(ctx context.Context) (*executor.Result, error) {
						cloneCtx, cancel := context.WithTimeout(ctx, time.Duration(cloneTimeout)*time.Second)
						defer cancel()
						if err := driver.Clone(cloneCtx, repo.URL, absPath, repo.Branch); err != nil {
							return nil, err
						}
						return &executor.Result{Success: true, Output: fmt.Sprintf("cloned to %s", repo.Path)}, nil
					},
				})
			}

			if isDryRun(cmd) || len(tasks) == 0 {
				return nil
			}

			fmt.Printf("Cloning %d repos using %s...\n", len(tasks), driver.Name())
			exec := executor.New(getParallelism(cmd), isQuiet(cmd), os.Stdout)
			results := exec.Run(context.Background(), tasks)

			if getOutputFormat(cmd) == "json" {
				env := output.NewEnvelope("clone", results)
				env.Errors = executor.ErrorSummary(results)
				env.Success = executor.CountErrors(results) == 0
				return output.New("json").Format(env)
			}

			errCount := executor.CountErrors(results)
			successCount := len(tasks) - errCount

			fmt.Println()
			if errCount > 0 {
				fmt.Fprintf(os.Stderr, "\033[31m%d repo(s) failed:\033[0m\n", errCount)
				for _, r := range results {
					if !r.Success {
						fmt.Fprintf(os.Stderr, "  \033[31m✗ %s\033[0m\n    %s\n", r.RepoName, r.Error)
					}
				}
				fmt.Println()
			}

			if successCount > 0 {
				fmt.Printf("\033[32m%d repo(s) cloned successfully\033[0m\n", successCount)
			}

			if errCount > 0 {
				return fmt.Errorf("%d repos failed to clone", errCount)
			}
			return nil
		},
	}
}

// isDirEmpty returns true if the directory exists and contains no entries.
func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}
