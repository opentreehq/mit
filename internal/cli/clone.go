package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/executor"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/vcs"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone all repos defined in mit.yaml",
	RunE:  runClone,
}

var cloneVCS string
var cloneTimeout int

func init() {
	cloneCmd.Flags().StringVar(&cloneVCS, "vcs", "git", "VCS driver to use for cloning: git or sl")
	cloneCmd.Flags().IntVar(&cloneTimeout, "timeout", 300, "timeout per repo in seconds")
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	driver, err := vcs.DriverByName(cloneVCS)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(flagRepos, flagExclude)
	repos := cfg.ResolveAll()

	var tasks []executor.Task
	for _, repo := range repos {
		repo := repo
		if !sel.Matches(repo.Name) {
			continue
		}

		absPath := filepath.Join(dir, repo.Path)
		if info, err := os.Stat(absPath); err == nil {
			if info.IsDir() && isDirEmpty(absPath) {
				// Empty directory — remove it so clone can proceed
				os.Remove(absPath)
			} else {
				if !flagQuiet {
					fmt.Printf("  %s already exists, skipping\n", repo.Name)
				}
				continue
			}
		}

		if flagDryRun {
			dryRunMsg("would clone %s to %s", repo.URL, repo.Path)
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

	if flagDryRun || len(tasks) == 0 {
		return nil
	}

	fmt.Printf("Cloning %d repos using %s...\n", len(tasks), driver.Name())
	exec := executor.New(getParallelism(), flagQuiet, os.Stdout)
	results := exec.Run(context.Background(), tasks)

	if getOutputFormat() == "json" {
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
}

// isDirEmpty returns true if the directory exists and contains no entries.
func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}
