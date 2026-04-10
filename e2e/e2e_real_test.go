//go:build !noembed

// Tests in this file use the real llama.cpp embedder.
// They are skipped if the model file is not present.
// Run with: go test ./internal/e2e/ -count=1 -v
package e2e

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabemeola/mit/embedding"
	"github.com/gabemeola/mit/index"
	"github.com/gabemeola/mit/statedb"
	"github.com/gabemeola/mit/workspace"
)

func skipIfNoModel(t *testing.T) {
	t.Helper()
	if !embedding.ModelExists(embedding.DefaultModelName) {
		t.Skipf("skipping: model %s not found (download to %s)",
			embedding.DefaultModelName, func() string { p, _ := embedding.ModelPath(embedding.DefaultModelName); return p }())
	}
}

func realEmbedder(t *testing.T) embedding.Embedder {
	t.Helper()
	skipIfNoModel(t)

	modelPath, err := embedding.ModelPath(embedding.DefaultModelName)
	if err != nil {
		t.Fatalf("model path: %v", err)
	}

	emb, err := embedding.NewEmbedder(modelPath, 4, -1)
	if err != nil {
		t.Fatalf("creating embedder: %v", err)
	}
	t.Cleanup(func() { emb.Close() })
	return emb
}

func TestReal_EmbedderBasic(t *testing.T) {
	emb := realEmbedder(t)

	dims := emb.Dimensions()
	if dims <= 0 {
		t.Fatalf("expected positive dimensions, got %d", dims)
	}
	t.Logf("model dimensions: %d", dims)

	vec, err := emb.Embed(context.Background(), "Hello world")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vec) != dims {
		t.Fatalf("vector length: got %d, want %d", len(vec), dims)
	}

	// Check it's normalized (L2 norm ≈ 1.0)
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("vector L2 norm: %f, expected ~1.0", norm)
	}
}

func TestReal_EmbedderDeterministic(t *testing.T) {
	emb := realEmbedder(t)

	text := "The quick brown fox jumps over the lazy dog"
	v1, _ := emb.Embed(context.Background(), text)
	v2, _ := emb.Embed(context.Background(), text)

	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("dimension %d differs: %f vs %f", i, v1[i], v2[i])
		}
	}
}

func TestReal_EmbedderSemanticSimilarity(t *testing.T) {
	emb := realEmbedder(t)
	ctx := context.Background()

	// Semantically similar texts
	v1, _ := emb.Embed(ctx, "How to authenticate users with OAuth2")
	v2, _ := emb.Embed(ctx, "User authentication using OAuth2 protocol")
	// Semantically different text
	v3, _ := emb.Embed(ctx, "Database migration scripts for PostgreSQL")

	simSimilar := index.CosineSimilarity(v1, v2)
	simDifferent := index.CosineSimilarity(v1, v3)

	t.Logf("similar pair:    %.4f", simSimilar)
	t.Logf("different pair:  %.4f", simDifferent)

	if simSimilar <= simDifferent {
		t.Errorf("expected similar texts (%.4f) to score higher than different texts (%.4f)",
			simSimilar, simDifferent)
	}
}

func TestReal_EmbedBatch(t *testing.T) {
	emb := realEmbedder(t)
	ctx := context.Background()

	texts := []string{
		"func handleLogin(user, pass string) error",
		"func connectDatabase(dsn string) (*sql.DB, error)",
		"func renderTemplate(w http.ResponseWriter, name string)",
	}

	batch, err := emb.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(batch) != 3 {
		t.Fatalf("batch length: got %d, want 3", len(batch))
	}

	// Each vector should be very close to individual Embed call
	// (not exact due to floating point non-determinism in multi-sequence batching)
	for i, text := range texts {
		single, _ := emb.Embed(ctx, text)
		sim := cosineSim(batch[i], single)
		if sim < 0.999 {
			t.Errorf("text %d: batch/single cosine similarity = %f (want > 0.999)", i, sim)
		}
	}
}

func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func TestReal_FullPipeline(t *testing.T) {
	emb := realEmbedder(t)

	root := setupWorkspace(t, map[string]map[string]string{
		"web-api": {
			"auth.go": `package api

// HandleLogin authenticates a user with username and password.
// It validates credentials against the database, generates a JWT token,
// and sets it as an HTTP-only cookie.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	user, err := db.ValidateCredentials(username, password)
	if err != nil {
		http.Error(w, "invalid credentials", 401)
		return
	}
	token := jwt.Generate(user.ID)
	http.SetCookie(w, &http.Cookie{Name: "token", Value: token})
}
`,
			"users.go": `package api

// ListUsers returns all users from the database with pagination.
// Supports offset/limit query parameters.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	users, err := db.GetUsers(offset, limit)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	json.NewEncoder(w).Encode(users)
}
`,
			"cache.go": `package api

// CacheMiddleware provides Redis-based HTTP response caching.
// It checks Redis for a cached response before hitting the handler.
func CacheMiddleware(ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "cache:" + r.URL.Path
			if cached, err := redis.Get(key); err == nil {
				w.Write(cached)
				return
			}
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			redis.Set(key, rec.Body.Bytes(), ttl)
		})
	}
}
`,
		},
	})

	ws, _ := workspace.Load(root)
	mitDir := filepath.Join(root, ".mit")
	os.MkdirAll(mitDir, 0755)
	db, _ := statedb.OpenPath(filepath.Join(mitDir, "state.db"))
	defer db.Close()

	indexer := index.NewIndexer(db, emb)
	ctx := context.Background()

	// Index
	repo := ws.Repos[0]
	stats, err := indexer.IndexRepo(ctx, repo.Name, repo.AbsPath)
	if err != nil {
		t.Fatalf("indexing: %v", err)
	}
	if stats.Indexed != 3 {
		t.Errorf("indexed: got %d, want 3", stats.Indexed)
	}

	// Test: auth-related query should rank auth.go highest
	results, err := indexer.Search(ctx, "user login authentication JWT token", 3)
	if err != nil {
		t.Fatalf("search auth: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results for auth query")
	}
	t.Logf("auth query results:")
	for _, r := range results {
		t.Logf("  %s:%s (%.4f)", r.Repo, r.File, r.Score)
	}
	if results[0].File != "auth.go" {
		t.Errorf("expected auth.go as top result, got %s", results[0].File)
	}

	// Test: cache-related query should rank cache.go highest
	results2, err := indexer.Search(ctx, "Redis caching HTTP response middleware", 3)
	if err != nil {
		t.Fatalf("search cache: %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("no results for cache query")
	}
	t.Logf("cache query results:")
	for _, r := range results2 {
		t.Logf("  %s:%s (%.4f)", r.Repo, r.File, r.Score)
	}
	if results2[0].File != "cache.go" {
		t.Errorf("expected cache.go as top result, got %s", results2[0].File)
	}

	// Test: database-related query should rank users.go highest
	results3, err := indexer.Search(ctx, "database query list users pagination", 3)
	if err != nil {
		t.Fatalf("search users: %v", err)
	}
	if len(results3) == 0 {
		t.Fatal("no results for users query")
	}
	t.Logf("users query results:")
	for _, r := range results3 {
		t.Logf("  %s:%s (%.4f)", r.Repo, r.File, r.Score)
	}
	if results3[0].File != "users.go" {
		t.Errorf("expected users.go as top result, got %s", results3[0].File)
	}
}
