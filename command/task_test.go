package command

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/opentreehq/mit/forge"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		// UUIDs get truncated to 8 chars
		{"abcdef01-2345-6789-abcd-ef0123456789", "abcdef01"},
		{"12345678", "12345678"},
		// Short IDs returned as-is
		{"abc", "abc"},
		{"", ""},
		// Remote IDs pass through unchanged
		{"github#42", "github#42"},
		{"gitlab#7", "gitlab#7"},
		{"github#999", "github#999"},
	}

	for _, tt := range tests {
		got := shortID(tt.id)
		if got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestIssueToTask(t *testing.T) {
	created := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	issue := forge.Issue{
		Number:    42,
		Title:     "Fix login bug",
		Body:      "Users can't log in",
		State:     "open",
		URL:       "https://github.com/org/repo/issues/42",
		Labels:    []string{"bug", "p1"},
		Author:    "alice",
		CreatedAt: created,
	}

	task := issueToTask(issue, "my-repo", forge.GitHub)

	if task.ID != "github#42" {
		t.Errorf("ID = %q, want %q", task.ID, "github#42")
	}
	if task.Title != "Fix login bug" {
		t.Errorf("Title = %q", task.Title)
	}
	if task.Description != "Users can't log in" {
		t.Errorf("Description = %q", task.Description)
	}
	if task.Status != "open" {
		t.Errorf("Status = %q", task.Status)
	}
	if task.Repo != "my-repo" {
		t.Errorf("Repo = %q", task.Repo)
	}
	if !task.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", task.CreatedAt, created)
	}

	// Metadata should be valid JSON with url, labels, author
	var meta map[string]any
	if err := json.Unmarshal([]byte(task.Metadata), &meta); err != nil {
		t.Fatalf("Metadata is not valid JSON: %v", err)
	}
	if meta["url"] != "https://github.com/org/repo/issues/42" {
		t.Errorf("Metadata url = %v", meta["url"])
	}
	if meta["author"] != "alice" {
		t.Errorf("Metadata author = %v", meta["author"])
	}
	labels, ok := meta["labels"].([]any)
	if !ok || len(labels) != 2 {
		t.Errorf("Metadata labels = %v", meta["labels"])
	}
}

func TestIssueToTask_GitLab(t *testing.T) {
	issue := forge.Issue{
		Number:    7,
		Title:     "MR feedback",
		State:     "opened",
		CreatedAt: time.Now(),
	}

	task := issueToTask(issue, "backend", forge.GitLab)

	if task.ID != "gitlab#7" {
		t.Errorf("ID = %q, want %q", task.ID, "gitlab#7")
	}
}
