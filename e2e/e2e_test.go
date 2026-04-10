// Package e2e contains end-to-end tests for the mit tool.
// These tests create real workspace directories, git repos, databases,
// and exercise the full pipeline: config → workspace → index → search.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabemeola/mit/config"
	"github.com/gabemeola/mit/index"
	"github.com/gabemeola/mit/statedb"
	"github.com/gabemeola/mit/workspace"
)

// --- Helpers ---

// setupWorkspace creates a mit workspace with real git repos.
func setupWorkspace(t *testing.T, repos map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()

	cfg := &config.Config{
		Version:   "1",
		Workspace: config.WorkspaceConfig{Name: "e2e-test"},
		Repos:     make(map[string]config.Repo),
	}

	for name, files := range repos {
		repoDir := filepath.Join(root, name)
		setupGitRepo(t, repoDir, files)
		cfg.Repos[name] = config.Repo{
			URL:    repoDir, // local path as URL
			Branch: "main",
		}
	}

	if err := config.Save(root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	return root
}

// setupGitRepo creates a real git repo with the given files committed.
func setupGitRepo(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	cmds := [][]string{
		{"git", "init", "--initial-branch=main"},
		{"git", "config", "user.email", "test@e2e.com"},
		{"git", "config", "user.name", "E2E Test"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git cmd %v: %v\n%s", args, err, out)
		}
	}
}

// mockEmbedder produces deterministic vectors based on text content hash.
type mockEmbedder struct {
	dims int
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dims)
	seed := uint32(0)
	for _, c := range text {
		seed = seed*31 + uint32(c)
	}
	for i := range vec {
		seed = seed*1103515245 + 12345
		vec[i] = float32(int32(seed>>16&0x7FFF)-16383) / 16383.0
	}
	// L2 normalize
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / (norm + 1e-12))
		}
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (m *mockEmbedder) Dimensions() int { return m.dims }
func (m *mockEmbedder) Close() error    { return nil }

// --- E2E Tests: Full Pipeline with Mock Embedder ---

func TestE2E_FullPipeline_IndexAndSearch(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"auth-service": {
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println("auth service starting")
	startAuthServer()
}

func startAuthServer() {
	// Initialize OAuth2 provider
	// Handle login, logout, token refresh
}
`,
			"handlers.go": `package main

func handleLogin(user, pass string) error {
	// Validate credentials against database
	// Generate JWT token
	return nil
}

func handleLogout(token string) error {
	// Invalidate session
	return nil
}
`,
		},
		"api-gateway": {
			"main.go": `package main

import "net/http"

func main() {
	http.HandleFunc("/api/v1/", routeRequest)
	http.ListenAndServe(":8080", nil)
}

func routeRequest(w http.ResponseWriter, r *http.Request) {
	// Route to appropriate microservice
	// Apply rate limiting
	// Check authentication
}
`,
			"middleware.go": `package main

func rateLimiter(next http.Handler) http.Handler {
	// Token bucket rate limiting
	return next
}

func authMiddleware(next http.Handler) http.Handler {
	// Verify JWT token from auth-service
	return next
}
`,
		},
	})

	// Load workspace
	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatalf("loading workspace: %v", err)
	}

	if len(ws.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(ws.Repos))
	}

	// Open database
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, err := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	// Index with mock embedder
	emb := &mockEmbedder{dims: 32}
	indexer := index.NewIndexer(db, emb)

	ctx := context.Background()
	totalIndexed := 0
	for _, repo := range ws.Repos {
		stats, err := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
		if err != nil {
			t.Fatalf("indexing %s: %v", repo.Name, err)
		}
		t.Logf("  %s: indexed=%d skipped=%d", repo.Name, stats.Indexed, stats.Unchanged)
		totalIndexed += stats.Indexed
	}

	if totalIndexed != 4 {
		t.Errorf("total indexed files: got %d, want 4", totalIndexed)
	}

	// Verify embeddings in DB
	records, err := db.GetAllEmbeddings()
	if err != nil {
		t.Fatalf("getting embeddings: %v", err)
	}
	if len(records) < 4 {
		t.Errorf("expected at least 4 embedding records, got %d", len(records))
	}

	// Search
	results, err := indexer.Search(ctx, "authentication login", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	// Results should reference our repos
	reposSeen := map[string]bool{}
	for _, r := range results {
		reposSeen[r.Repo] = true
		if r.File == "" {
			t.Error("result has empty file")
		}
	}
	if !reposSeen["auth-service"] && !reposSeen["api-gateway"] {
		t.Error("expected results from at least one of our repos")
	}
}

func TestE2E_IncrementalReindex(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"myrepo": {
			"server.go": `package main

func startServer() {
	// HTTP server setup
}
`,
			"db.go": `package main

func connectDB() {
	// Database connection
}
`,
		},
	})

	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatalf("loading workspace: %v", err)
	}

	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, err := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	emb := &mockEmbedder{dims: 16}
	indexer := index.NewIndexer(db, emb)
	ctx := context.Background()

	repo := ws.Repos[0]

	// First index
	stats1, _ := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if stats1.Indexed != 2 {
		t.Errorf("first pass indexed: got %d, want 2", stats1.Indexed)
	}
	if stats1.Unchanged != 0 {
		t.Errorf("first pass skipped: got %d, want 0", stats1.Unchanged)
	}

	// Second index — no changes, all skipped
	stats2, _ := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if stats2.Indexed != 0 {
		t.Errorf("second pass indexed: got %d, want 0", stats2.Indexed)
	}
	if stats2.Unchanged != 2 {
		t.Errorf("second pass skipped: got %d, want 2", stats2.Unchanged)
	}

	// Modify one file
	serverPath := filepath.Join(repo.AbsPath, "server.go")
	os.WriteFile(serverPath, []byte(`package main

func startServer() {
	// HTTP server with TLS
	// Added TLS support
}
`), 0644)

	// Third index — one changed, one skipped
	stats3, _ := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if stats3.Indexed != 1 {
		t.Errorf("third pass indexed: got %d, want 1", stats3.Indexed)
	}
	if stats3.Unchanged != 1 {
		t.Errorf("third pass skipped: got %d, want 1", stats3.Unchanged)
	}

	// Add a new file
	os.WriteFile(filepath.Join(repo.AbsPath, "cache.go"), []byte(`package main

func initCache() {
	// Redis cache setup
}
`), 0644)

	// Fourth index — new file indexed, old two skipped
	stats4, _ := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if stats4.Indexed != 1 {
		t.Errorf("fourth pass indexed: got %d, want 1", stats4.Indexed)
	}
	if stats4.Unchanged != 2 {
		t.Errorf("fourth pass skipped: got %d, want 2", stats4.Unchanged)
	}
}

func TestE2E_MultiRepoSearch(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"frontend": {
			"app.tsx": `import React from 'react';

export function App() {
	return <div>Hello World</div>;
}
`,
			"api.ts": `export async function fetchUsers() {
	return fetch('/api/users').then(r => r.json());
}
`,
		},
		"backend": {
			"users.go": `package api

func ListUsers(w http.ResponseWriter, r *http.Request) {
	users, _ := db.Query("SELECT * FROM users")
	json.NewEncoder(w).Encode(users)
}
`,
			"db.go": `package api

import "database/sql"

func ConnectDB(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}
`,
		},
		"shared": {
			"types.ts": `export interface User {
	id: string;
	name: string;
	email: string;
}
`,
		},
	})

	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatalf("loading workspace: %v", err)
	}

	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, err := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	emb := &mockEmbedder{dims: 32}
	indexer := index.NewIndexer(db, emb)
	ctx := context.Background()

	// Index all repos
	for _, repo := range ws.Repos {
		_, err := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
		if err != nil {
			t.Fatalf("indexing %s: %v", repo.Name, err)
		}
	}

	// Verify all repos are indexed
	records, _ := db.GetAllEmbeddings()
	repos := map[string]int{}
	for _, r := range records {
		repos[r.Repo]++
	}
	if len(repos) != 3 {
		t.Errorf("expected 3 repos in index, got %d: %v", len(repos), repos)
	}

	// Search across all repos
	results, err := indexer.Search(ctx, "database query users", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	// Should have results from multiple repos
	resultRepos := map[string]bool{}
	for _, r := range results {
		resultRepos[r.Repo] = true
	}
	if len(resultRepos) < 2 {
		t.Errorf("expected results from multiple repos, got repos: %v", resultRepos)
	}
}

func TestE2E_WorkspaceLifecycle(t *testing.T) {
	root := t.TempDir()

	// 1. Create config manually (simulating "mit init" + "mit add")
	cfg := &config.Config{
		Version:   "1",
		Workspace: config.WorkspaceConfig{Name: "lifecycle-test"},
		Repos:     make(map[string]config.Repo),
	}

	repoDir := filepath.Join(root, "my-service")
	setupGitRepo(t, repoDir, map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"config.yaml": "port: 8080\n",
	})

	cfg.Repos["my-service"] = config.Repo{
		URL:    repoDir,
		Branch: "main",
	}

	if err := config.Save(root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// 2. Load workspace
	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatalf("loading workspace: %v", err)
	}
	if ws.Config.Workspace.Name != "lifecycle-test" {
		t.Errorf("workspace name: got %q", ws.Config.Workspace.Name)
	}

	// 3. Open DB + index
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, err := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	emb := &mockEmbedder{dims: 8}
	indexer := index.NewIndexer(db, emb)
	ctx := context.Background()

	repo := ws.Repos[0]
	stats, err := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if err != nil {
		t.Fatalf("indexing: %v", err)
	}
	// main.go and config.yaml should both be indexed
	if stats.Indexed != 2 {
		t.Errorf("indexed: got %d, want 2", stats.Indexed)
	}

	// 4. Search
	results, err := indexer.Search(ctx, "configuration", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	// 5. Create a task in the same DB (testing task + index coexistence)
	taskID, err := db.CreateTask("Review search results", "Check relevance", "my-service")
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	task, err := db.GetTask(taskID)
	if err != nil {
		t.Fatalf("getting task: %v", err)
	}
	if task.Title != "Review search results" {
		t.Errorf("task title: got %q", task.Title)
	}

	// 6. Verify index state persists
	state, _ := db.GetIndexState(filepath.Join(repo.Name, "main.go"))
	if state.Checksum == "" {
		t.Error("expected non-empty checksum in index state")
	}
}

func TestE2E_IndexSkipsNonSourceFiles(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"mixed-repo": {
			"src/app.go":          "package app\n\nfunc Run() {}\n",
			"src/app_test.go":     "package app\n\nfunc TestRun(t *testing.T) {}\n",
			"docs/README.md":      "# Documentation\n\nHow to use this.\n",
			"assets/logo.png":     "fake png data",
			"assets/icon.svg":     "<svg></svg>",
			"dist/bundle.js.map":  "source map data",
			"node_modules/x/y.js": "module code",
			"vendor/lib/z.go":     "package lib",
		},
	})

	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatalf("loading workspace: %v", err)
	}

	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, err := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	emb := &mockEmbedder{dims: 8}
	indexer := index.NewIndexer(db, emb)

	repo := ws.Repos[0]
	stats, err := indexer.IndexRepo(context.Background(), repo.Name, repo.AbsPath)
	if err != nil {
		t.Fatalf("indexing: %v", err)
	}

	// Should index: app.go, app_test.go, README.md
	// Should skip: logo.png (not indexable), icon.svg (not indexable),
	//   bundle.js.map (not indexable), node_modules/ (skipped dir),
	//   vendor/ (skipped dir), .git/ (skipped dir)
	if stats.Indexed != 3 {
		t.Errorf("indexed: got %d, want 3", stats.Indexed)
	}

	records, _ := db.GetAllEmbeddings()
	files := map[string]bool{}
	for _, r := range records {
		files[r.File] = true
	}

	for _, expected := range []string{"src/app.go", "src/app_test.go", "docs/README.md"} {
		if !files[expected] {
			t.Errorf("expected %q to be indexed, files: %v", expected, files)
		}
	}

	for _, excluded := range []string{"assets/logo.png", "node_modules/x/y.js", "vendor/lib/z.go"} {
		if files[excluded] {
			t.Errorf("expected %q to be excluded from index", excluded)
		}
	}
}

func TestE2E_LargeFileChunking(t *testing.T) {
	// Create a file large enough to be split into multiple chunks
	var lines strings.Builder
	for i := 0; i < 350; i++ {
		lines.WriteString("func handler" + string(rune('A'+i%26)) + "() { /* handler logic */ }\n")
	}

	root := setupWorkspace(t, map[string]map[string]string{
		"big-repo": {
			"handlers.go": lines.String(),
		},
	})

	ws, _ := workspace.Load(root)
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, _ := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	defer db.Close()

	emb := &mockEmbedder{dims: 16}
	indexer := index.NewIndexer(db, emb)

	repo := ws.Repos[0]
	stats, err := indexer.IndexRepo(context.Background(), repo.Name, repo.AbsPath)
	if err != nil {
		t.Fatalf("indexing: %v", err)
	}
	if stats.Indexed != 1 {
		t.Errorf("indexed files: got %d, want 1", stats.Indexed)
	}

	// 350 lines / 50-line chunks = 7 chunks
	records, _ := db.GetAllEmbeddings()
	if len(records) != 7 {
		t.Errorf("chunks: got %d, want 7", len(records))
	}

	// Verify chunk metadata
	for i, r := range records {
		if r.ChunkIndex != i {
			t.Errorf("chunk %d: got index %d", i, r.ChunkIndex)
		}
		if r.LineStart < 1 {
			t.Errorf("chunk %d: line_start=%d, want >= 1", i, r.LineStart)
		}
		if i > 0 && r.LineStart != records[i-1].LineEnd+1 {
			t.Errorf("chunk %d: gap between line_end=%d and line_start=%d",
				i, records[i-1].LineEnd, r.LineStart)
		}
	}

	// Search should still work across chunks
	results, err := indexer.Search(context.Background(), "handler logic", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results from chunked file")
	}
}

func TestE2E_SearchRanking(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"repo": {
			"auth.go":    "package main\n\n// Authentication and authorization logic\nfunc authenticate() {}\nfunc authorize() {}\n",
			"logging.go": "package main\n\n// Structured logging utilities\nfunc logInfo() {}\nfunc logError() {}\n",
			"cache.go":   "package main\n\n// Redis cache layer\nfunc getCache() {}\nfunc setCache() {}\n",
		},
	})

	ws, _ := workspace.Load(root)
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, _ := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	defer db.Close()

	emb := &mockEmbedder{dims: 32}
	indexer := index.NewIndexer(db, emb)
	ctx := context.Background()

	repo := ws.Repos[0]
	indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)

	results, err := indexer.Search(ctx, "auth", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	// Should return exactly 3 results (all files)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d].Score=%f > [%d].Score=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestE2E_DatabasePersistence(t *testing.T) {
	root := setupWorkspace(t, map[string]map[string]string{
		"repo": {
			"main.go": "package main\n\nfunc main() {}\n",
		},
	})

	ws, _ := workspace.Load(root)
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	dbPath := filepath.Join(mitDir, "state.db")

	// First session: index
	{
		db, _ := statedb.OpenPath(dbPath)
		emb := &mockEmbedder{dims: 8}
		indexer := index.NewIndexer(db, emb)
		indexer.IndexRepo(context.Background(), ws.Repos[0].Name, ws.Repos[0].AbsPath)
		db.Close()
	}

	// Second session: verify data persists and search works
	{
		db, _ := statedb.OpenPath(dbPath)
		defer db.Close()

		records, _ := db.GetAllEmbeddings()
		if len(records) == 0 {
			t.Fatal("embeddings should persist across DB sessions")
		}

		emb := &mockEmbedder{dims: 8}
		indexer := index.NewIndexer(db, emb)

		// Index state should persist — file should be skipped
		stats, _ := indexer.IndexRepo(context.Background(), ws.Repos[0].Name, ws.Repos[0].AbsPath)
		if stats.Unchanged != 1 {
			t.Errorf("expected file to be skipped on re-open, got skipped=%d", stats.Unchanged)
		}

		// Search should work with persisted embeddings
		results, err := indexer.Search(context.Background(), "main function", 5)
		if err != nil {
			t.Fatalf("search after reopen: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected search results from persisted data")
		}
	}
}
