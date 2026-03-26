package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return r
}

func TestCreateAndGet(t *testing.T) {
	r := setupTestRegistry(t)

	skill := &Skill{
		Name:        "deploy",
		Description: "Deploy services to production",
		Triggers:    []string{"deploy", "release", "ship"},
		Repos:       []string{"api", "web"},
		Content:     "Run `make deploy` in each repo in order: api, then web.",
	}

	if err := r.Create(skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.Get("deploy")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Name != "deploy" {
		t.Errorf("name: got %q, want %q", got.Name, "deploy")
	}
	if got.Description != skill.Description {
		t.Errorf("description mismatch")
	}
	if len(got.Triggers) != 3 {
		t.Errorf("expected 3 triggers, got %d", len(got.Triggers))
	}
	if got.Content != skill.Content {
		t.Errorf("content: got %q, want %q", got.Content, skill.Content)
	}
}

func TestCreateDuplicate(t *testing.T) {
	r := setupTestRegistry(t)

	skill := &Skill{Name: "test", Description: "test skill", Content: "test"}
	if err := r.Create(skill); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.Create(skill); err == nil {
		t.Fatal("expected error creating duplicate skill")
	}
}

func TestCreateEmptyName(t *testing.T) {
	r := setupTestRegistry(t)
	skill := &Skill{Description: "no name", Content: "test"}
	if err := r.Create(skill); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestList(t *testing.T) {
	r := setupTestRegistry(t)

	skills := []Skill{
		{Name: "deploy", Description: "Deploy", Content: "deploy steps"},
		{Name: "test", Description: "Run tests", Content: "test steps"},
		{Name: "lint", Description: "Run linting", Content: "lint steps"},
	}
	for i := range skills {
		if err := r.Create(&skills[i]); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	all, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 skills, got %d", len(all))
	}
}

func TestSearch(t *testing.T) {
	r := setupTestRegistry(t)

	skills := []Skill{
		{Name: "deploy", Description: "Deploy services", Triggers: []string{"release"}, Content: "deploy"},
		{Name: "test", Description: "Run unit tests", Triggers: []string{"verify"}, Content: "test"},
		{Name: "db-migrate", Description: "Run database migrations", Triggers: []string{"migrate", "deploy"}, Content: "migrate"},
	}
	for i := range skills {
		if err := r.Create(&skills[i]); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Search by name
	results, err := r.Search("deploy")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 { // "deploy" name + "db-migrate" has deploy trigger
		t.Errorf("expected 2 results for 'deploy', got %d", len(results))
	}

	// Search by description
	results, err = r.Search("database")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'database', got %d", len(results))
	}

	// No matches
	results, err = r.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFileFormat(t *testing.T) {
	r := setupTestRegistry(t)

	skill := &Skill{
		Name:        "build",
		Description: "Build all services",
		Triggers:    []string{"compile"},
		Content:     "Run make build in each repo",
	}
	if err := r.Create(skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(r.dir, "build.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !containsStr(content, "---") {
		t.Error("expected YAML frontmatter delimiters")
	}
	if !containsStr(content, "name: build") {
		t.Error("expected name in frontmatter")
	}
	if !containsStr(content, "Run make build") {
		t.Error("expected content body")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"deploy", "deploy"},
		{"db migrate", "db-migrate"},
		{"My Skill!", "my-skill"},
		{"test_123", "test_123"},
	}
	for _, tt := range tests {
		got := sanitizeName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
