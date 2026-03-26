package executor

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
)

// TaskFunc is a function that performs work for a single repo.
type TaskFunc func(ctx context.Context) (*Result, error)

// Task represents a unit of work to execute.
type Task struct {
	RepoName string
	Fn       TaskFunc
}

// Executor runs tasks in parallel with bounded concurrency.
type Executor struct {
	parallelism int
	quiet       bool
	w           io.Writer
}

// New creates a new executor with the given parallelism.
func New(parallelism int, quiet bool, w io.Writer) *Executor {
	return &Executor{
		parallelism: parallelism,
		quiet:       quiet,
		w:           w,
	}
}

// Run executes all tasks with bounded concurrency and returns results in order.
func (e *Executor) Run(ctx context.Context, tasks []Task) []Result {
	results := make([]Result, len(tasks))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(e.parallelism)

	completed := 0
	total := len(tasks)

	for i, task := range tasks {
		i, task := i, task
		g.Go(func() error {
			start := time.Now()
			result, err := task.Fn(ctx)
			elapsed := time.Since(start)

			mu.Lock()
			completed++
			done := completed
			mu.Unlock()

			if err != nil {
				results[i] = Result{
					RepoName: task.RepoName,
					Success:  false,
					Error:    err.Error(),
				}
				if !e.quiet {
					mu.Lock()
					fmt.Fprintf(e.w, "  %s%s FAILED%s (%s) [%d/%d]\n", colorRed, task.RepoName, colorReset, formatDuration(elapsed), done, total)
					mu.Unlock()
				}
				return nil // Don't fail the group, collect all results
			}

			if result == nil {
				result = &Result{
					RepoName: task.RepoName,
					Success:  true,
				}
			}
			result.RepoName = task.RepoName
			results[i] = *result

			if !e.quiet {
				mu.Lock()
				fmt.Fprintf(e.w, "  %s%s ok%s (%s) [%d/%d]\n", colorGreen, task.RepoName, colorReset, formatDuration(elapsed), done, total)
				mu.Unlock()
			}

			return nil
		})
	}

	g.Wait()
	return results
}

// RunSimple executes a simple function for each repo name.
func (e *Executor) RunSimple(ctx context.Context, repoNames []string, fn func(ctx context.Context, name string) error) []Result {
	tasks := make([]Task, len(repoNames))
	for i, name := range repoNames {
		name := name
		tasks[i] = Task{
			RepoName: name,
			Fn: func(ctx context.Context) (*Result, error) {
				if err := fn(ctx, name); err != nil {
					return nil, err
				}
				return &Result{Success: true}, nil
			},
		}
	}
	return e.Run(ctx, tasks)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fm%.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
}

// CountErrors returns the number of failed results.
func CountErrors(results []Result) int {
	count := 0
	for _, r := range results {
		if !r.Success {
			count++
		}
	}
	return count
}

// ErrorSummary returns a summary of all errors.
func ErrorSummary(results []Result) []string {
	var errors []string
	for _, r := range results {
		if !r.Success {
			errors = append(errors, fmt.Sprintf("%s: %s", r.RepoName, r.Error))
		}
	}
	return errors
}
