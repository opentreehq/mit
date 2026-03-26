package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestAddAndGet(t *testing.T) {
	s := setupTestStore(t)

	mem := &Memory{
		Type:      "observation",
		Repo:      "api",
		Tags:      []string{"perf", "db"},
		CreatedBy: "test",
		Content:   "Database queries are slow under load",
	}

	if err := s.Add(mem); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if mem.ID == "" {
		t.Fatal("expected ID to be generated")
	}
	if mem.CreatedAt == "" {
		t.Fatal("expected CreatedAt to be set")
	}

	got, err := s.Get(mem.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Content != mem.Content {
		t.Errorf("content mismatch: got %q, want %q", got.Content, mem.Content)
	}
	if got.Type != "observation" {
		t.Errorf("type mismatch: got %q, want %q", got.Type, "observation")
	}
	if got.Repo != "api" {
		t.Errorf("repo mismatch: got %q, want %q", got.Repo, "api")
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
}

func TestAddInvalidType(t *testing.T) {
	s := setupTestStore(t)

	mem := &Memory{
		Type:    "invalid",
		Content: "test",
	}
	if err := s.Add(mem); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestList(t *testing.T) {
	s := setupTestStore(t)

	mems := []Memory{
		{Type: "observation", Repo: "api", Content: "first"},
		{Type: "decision", Repo: "web", Content: "second"},
		{Type: "observation", Repo: "web", Content: "third"},
	}
	for i := range mems {
		if err := s.Add(&mems[i]); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	// List all
	all, err := s.List("", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	// Filter by type
	obs, err := s.List("observation", "")
	if err != nil {
		t.Fatalf("List by type: %v", err)
	}
	if len(obs) != 2 {
		t.Errorf("expected 2 observations, got %d", len(obs))
	}

	// Filter by repo
	web, err := s.List("", "web")
	if err != nil {
		t.Fatalf("List by repo: %v", err)
	}
	if len(web) != 2 {
		t.Errorf("expected 2 web memories, got %d", len(web))
	}

	// Filter by both
	both, err := s.List("observation", "web")
	if err != nil {
		t.Fatalf("List by both: %v", err)
	}
	if len(both) != 1 {
		t.Errorf("expected 1, got %d", len(both))
	}
}

func TestRemove(t *testing.T) {
	s := setupTestStore(t)

	mem := &Memory{Type: "pattern", Content: "to be removed"}
	if err := s.Add(mem); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Remove(mem.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Should not exist on disk
	path := filepath.Join(s.dir, mem.ID+".md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected file to be deleted")
	}

	// Remove again should error
	if err := s.Remove(mem.ID); err == nil {
		t.Fatal("expected error removing non-existent memory")
	}
}

func TestSearch(t *testing.T) {
	s := setupTestStore(t)

	mems := []Memory{
		{Type: "observation", Tags: []string{"performance"}, Content: "API latency is high"},
		{Type: "decision", Tags: []string{"architecture"}, Content: "Use Redis for caching"},
		{Type: "gotcha", Tags: []string{"performance", "redis"}, Content: "Redis connection pool must be tuned"},
	}
	for i := range mems {
		if err := s.Add(&mems[i]); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	// Search by content keyword
	results, err := s.Search("redis")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'redis', got %d", len(results))
	}

	// Search by tag
	results, err = s.Search("performance")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'performance', got %d", len(results))
	}

	// No matches
	results, err = s.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFileFormat(t *testing.T) {
	s := setupTestStore(t)

	mem := &Memory{
		ID:        "test-id",
		Type:      "decision",
		Repo:      "core",
		Tags:      []string{"arch"},
		CreatedBy: "alice",
		Content:   "We chose PostgreSQL for persistence",
	}
	if err := s.Add(mem); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.dir, "test-id.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !contains(content, "---") {
		t.Error("expected YAML frontmatter delimiters")
	}
	if !contains(content, "type: decision") {
		t.Error("expected type in frontmatter")
	}
	if !contains(content, "We chose PostgreSQL") {
		t.Error("expected content body")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
