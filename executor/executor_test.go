package executor

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestExecutor_RunParallel(t *testing.T) {
	var buf bytes.Buffer
	exec := New(4, true, &buf)

	var count atomic.Int32
	tasks := make([]Task, 10)
	for i := range tasks {
		i := i
		tasks[i] = Task{
			RepoName: fmt.Sprintf("repo-%d", i),
			Fn: func(ctx context.Context) (*Result, error) {
				count.Add(1)
				return &Result{Success: true, Output: fmt.Sprintf("done-%d", i)}, nil
			},
		}
	}

	results := exec.Run(context.Background(), tasks)

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	if count.Load() != 10 {
		t.Fatalf("expected 10 tasks executed, got %d", count.Load())
	}

	for i, r := range results {
		if !r.Success {
			t.Errorf("result %d: expected success", i)
		}
		if r.RepoName != fmt.Sprintf("repo-%d", i) {
			t.Errorf("result %d: expected repo name 'repo-%d', got %q", i, i, r.RepoName)
		}
	}
}

func TestExecutor_RunWithErrors(t *testing.T) {
	var buf bytes.Buffer
	exec := New(2, true, &buf)

	tasks := []Task{
		{
			RepoName: "good",
			Fn: func(ctx context.Context) (*Result, error) {
				return &Result{Success: true}, nil
			},
		},
		{
			RepoName: "bad",
			Fn: func(ctx context.Context) (*Result, error) {
				return nil, fmt.Errorf("something failed")
			},
		},
	}

	results := exec.Run(context.Background(), tasks)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Success {
		t.Error("first result should be success")
	}
	if results[1].Success {
		t.Error("second result should be failure")
	}
	if results[1].Error != "something failed" {
		t.Errorf("expected error 'something failed', got %q", results[1].Error)
	}
}

func TestExecutor_RunSimple(t *testing.T) {
	var buf bytes.Buffer
	exec := New(2, true, &buf)

	names := []string{"a", "b", "c"}
	var executed []string
	results := exec.RunSimple(context.Background(), names, func(ctx context.Context, name string) error {
		executed = append(executed, name)
		return nil
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if CountErrors(results) != 0 {
		t.Errorf("expected 0 errors, got %d", CountErrors(results))
	}
}

func TestCountErrors(t *testing.T) {
	results := []Result{
		{Success: true},
		{Success: false, Error: "err1"},
		{Success: true},
		{Success: false, Error: "err2"},
	}
	if got := CountErrors(results); got != 2 {
		t.Errorf("CountErrors = %d, want 2", got)
	}
}

func TestErrorSummary(t *testing.T) {
	results := []Result{
		{RepoName: "a", Success: true},
		{RepoName: "b", Success: false, Error: "fail"},
	}
	summary := ErrorSummary(results)
	if len(summary) != 1 {
		t.Fatalf("expected 1 error summary, got %d", len(summary))
	}
	if summary[0] != "b: fail" {
		t.Errorf("got %q", summary[0])
	}
}
