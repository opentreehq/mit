package index

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabemeola/mit/internal/statedb"
)

// mockEmbedder returns deterministic embeddings based on text length.
type mockEmbedder struct {
	dims      int
	callCount int
}

func newMockEmbedder(dims int) *mockEmbedder {
	return &mockEmbedder{dims: dims}
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	vec := make([]float32, m.dims)
	// Create a deterministic embedding based on text content.
	// Use a simple hash-like approach so different texts get different directions.
	seed := uint32(0)
	for _, c := range text {
		seed = seed*31 + uint32(c)
	}
	for i := range vec {
		seed = seed*1103515245 + 12345
		vec[i] = float32(int32(seed>>16&0x7FFF)-16383) / 16383.0
	}
	// Normalize
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := m.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *mockEmbedder) Dimensions() int { return m.dims }
func (m *mockEmbedder) Close() error    { return nil }

// --- Helper ---

func tempDB(t *testing.T) *statedb.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := statedb.OpenPath(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("opening temp db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

// --- IndexRepo tests ---

func TestIndexer_IndexRepo_Basic(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	repoDir := createTestRepo(t, map[string]string{
		"main.go":       "package main\n\nfunc main() {}\n",
		"util.go":       "package main\n\nfunc helper() {}\n",
		"README.md":     "# Test\n",
		"image.png":     "binary data",
		".git/HEAD":     "ref: refs/heads/main",
		"node_modules/a": "skip me",
	})

	stats, err := indexer.IndexRepo(context.Background(), "test-repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}

	// Should index .go and .md files, skip .png, skip .git/ and node_modules/
	if stats.Indexed != 3 {
		t.Errorf("indexed: got %d, want 3", stats.Indexed)
	}
	if stats.Unchanged != 0 {
		t.Errorf("skipped: got %d, want 0", stats.Unchanged)
	}

	// Verify embeddings stored
	records, _ := db.GetAllEmbeddings()
	if len(records) < 3 {
		t.Errorf("expected at least 3 embedding records, got %d", len(records))
	}

	// Verify embedder was called
	if emb.callCount < 3 {
		t.Errorf("embedder called %d times, expected at least 3", emb.callCount)
	}
}

func TestIndexer_IndexRepo_Incremental(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	repoDir := createTestRepo(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	// First index
	stats1, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("first IndexRepo: %v", err)
	}
	if stats1.Indexed != 1 || stats1.Unchanged != 0 {
		t.Errorf("first pass: indexed=%d skipped=%d, want 1/0", stats1.Indexed, stats1.Unchanged)
	}
	callsAfterFirst := emb.callCount

	// Second index without changes - should skip
	stats2, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("second IndexRepo: %v", err)
	}
	if stats2.Indexed != 0 {
		t.Errorf("second pass indexed: got %d, want 0", stats2.Indexed)
	}
	if stats2.Unchanged != 1 {
		t.Errorf("second pass skipped: got %d, want 1", stats2.Unchanged)
	}
	if emb.callCount != callsAfterFirst {
		t.Error("embedder should not have been called on unchanged files")
	}
}

func TestIndexer_IndexRepo_ReindexOnChange(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	repoDir := createTestRepo(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	// First index
	indexer.IndexRepo(context.Background(), "repo", repoDir)
	callsAfterFirst := emb.callCount

	records1, _ := db.GetAllEmbeddings()
	count1 := len(records1)

	// Modify the file
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"changed\")\n}\n"), 0644)

	// Re-index - should process the changed file
	stats, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("re-index: %v", err)
	}
	if stats.Indexed != 1 {
		t.Errorf("re-index: got %d indexed, want 1", stats.Indexed)
	}
	if stats.Unchanged != 0 {
		t.Errorf("re-index: got %d skipped, want 0", stats.Unchanged)
	}
	if emb.callCount <= callsAfterFirst {
		t.Error("embedder should have been called for changed file")
	}

	// Should have replaced old embeddings (same count)
	records2, _ := db.GetAllEmbeddings()
	if len(records2) != count1 {
		t.Errorf("expected %d records after re-index, got %d", count1, len(records2))
	}
}

func TestIndexer_IndexRepo_EmptyRepo(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	repoDir := t.TempDir() // empty directory

	stats, err := indexer.IndexRepo(context.Background(), "empty", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo on empty: %v", err)
	}
	if stats.Indexed != 0 || stats.Unchanged != 0 {
		t.Errorf("empty repo: indexed=%d skipped=%d, want 0/0", stats.Indexed, stats.Unchanged)
	}
}

func TestIndexer_IndexRepo_LargeFileChunking(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	// Create a file with 250 lines (should produce 3 chunks at 100 lines each)
	var content string
	for i := 0; i < 250; i++ {
		content += "line of code\n"
	}
	repoDir := createTestRepo(t, map[string]string{
		"big.go": content,
	})

	stats, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}
	if stats.Indexed != 1 {
		t.Errorf("indexed files: got %d, want 1", stats.Indexed)
	}

	// Should have 5 chunks stored (250 lines / 50 lines per chunk)
	records, _ := db.GetAllEmbeddings()
	if len(records) != 5 {
		t.Errorf("chunks stored: got %d, want 5", len(records))
	}

	// Verify chunk indices
	for i, r := range records {
		if r.ChunkIndex != i {
			t.Errorf("chunk %d: got index %d", i, r.ChunkIndex)
		}
	}
}

// --- Failing embedder mocks ---

// failingBatchEmbedder returns an error from EmbedBatch (simulates embedder
// closed or context cancelled — the only cases where EmbedBatch returns error).
type failingBatchEmbedder struct {
	dims int
}

func (f *failingBatchEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return make([]float32, f.dims), nil
}
func (f *failingBatchEmbedder) EmbedBatch(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedder is closed")
}
func (f *failingBatchEmbedder) Dimensions() int { return f.dims }
func (f *failingBatchEmbedder) Close() error    { return nil }

// zeroVectorBatchEmbedder returns real embeddings for some chunks but zero
// vectors for others (simulates the llama.go fallback behavior where decode
// fails for certain inputs).
type zeroVectorBatchEmbedder struct {
	dims          int
	inner         *mockEmbedder
	zeroEveryNth  int // produce zero vector for every Nth chunk (1-indexed)
}

func newZeroVectorBatchEmbedder(dims, zeroEveryNth int) *zeroVectorBatchEmbedder {
	return &zeroVectorBatchEmbedder{
		dims:         dims,
		inner:        newMockEmbedder(dims),
		zeroEveryNth: zeroEveryNth,
	}
}

func (z *zeroVectorBatchEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return z.inner.Embed(ctx, text)
}

func (z *zeroVectorBatchEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		if (i+1)%z.zeroEveryNth == 0 {
			results[i] = make([]float32, z.dims) // zero vector
		} else {
			emb, _ := z.inner.Embed(ctx, t)
			results[i] = emb
		}
	}
	return results, nil
}

func (z *zeroVectorBatchEmbedder) Dimensions() int { return z.dims }
func (z *zeroVectorBatchEmbedder) Close() error    { return nil }

// shortBatchEmbedder returns fewer embeddings than texts (simulates partial
// batch where not all sequences fit in the context window).
type shortBatchEmbedder struct {
	dims   int
	inner  *mockEmbedder
	maxFit int // max number of embeddings returned per batch
}

func (s *shortBatchEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return s.inner.Embed(ctx, text)
}

func (s *shortBatchEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	n := len(texts)
	if n > s.maxFit {
		n = s.maxFit
	}
	results := make([][]float32, n)
	for i := 0; i < n; i++ {
		emb, _ := s.inner.Embed(ctx, texts[i])
		results[i] = emb
	}
	return results, nil
}

func (s *shortBatchEmbedder) Dimensions() int { return s.dims }
func (s *shortBatchEmbedder) Close() error    { return nil }

// --- Failure-mode tests ---

func TestIndexer_IndexRepo_BatchEmbedFailure(t *testing.T) {
	// When EmbedBatch returns an error (embedder closed / context cancelled),
	// the indexer should skip the file gracefully — no panic, no SkippedTooLarge.
	db := tempDB(t)
	emb := &failingBatchEmbedder{dims: 8}
	indexer := NewIndexer(db, emb)

	repoDir := createTestRepo(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
		"util.go": "package main\n\nfunc helper() {}\n",
	})

	stats, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo should not return error: %v", err)
	}

	// Files should be skipped (EmbedBatch error → continue), not indexed
	if stats.Indexed != 0 {
		t.Errorf("indexed: got %d, want 0 (batch failed, files should be skipped)", stats.Indexed)
	}

	// No embeddings should be stored
	records, _ := db.GetAllEmbeddings()
	if len(records) != 0 {
		t.Errorf("expected 0 stored embeddings after batch failure, got %d", len(records))
	}
}

func TestIndexer_IndexRepo_ZeroVectorChunks(t *testing.T) {
	// When EmbedBatch returns zero vectors for some chunks (fallback behavior),
	// ALL chunks should still be stored — zero vectors rank low in search
	// but don't cause data loss.
	db := tempDB(t)
	emb := newZeroVectorBatchEmbedder(8, 2) // zero-vector every 2nd chunk
	indexer := NewIndexer(db, emb)

	// Create a file large enough to produce multiple chunks
	var content string
	for i := 0; i < 150; i++ {
		content += "line of code\n"
	}
	repoDir := createTestRepo(t, map[string]string{
		"big.go": content,
	})

	stats, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}

	if stats.Indexed != 1 {
		t.Errorf("indexed: got %d, want 1", stats.Indexed)
	}

	// All chunks should be stored (150 lines / 50 lines per chunk = 3 chunks)
	records, _ := db.GetAllEmbeddings()
	if len(records) != 3 {
		t.Errorf("stored chunks: got %d, want 3 (zero-vector chunks should still be stored)", len(records))
	}

	// Verify that zero-vector embeddings are actually stored (not silently dropped)
	zeroCount := 0
	for _, r := range records {
		embs := BytesToFloat32(r.Embedding)
		allZero := true
		for _, v := range embs {
			if v != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			zeroCount++
		}
	}
	// Every 2nd chunk (chunks at index 1) should be a zero vector
	if zeroCount == 0 {
		t.Error("expected at least one zero-vector embedding to be stored")
	}
}

func TestIndexer_IndexRepo_ShortBatch(t *testing.T) {
	// When EmbedBatch returns fewer results than chunks (partial batch fit),
	// the indexer should store only the returned embeddings without panicking.
	db := tempDB(t)
	emb := &shortBatchEmbedder{dims: 8, inner: newMockEmbedder(8), maxFit: 1}
	indexer := NewIndexer(db, emb)

	// Create a file that produces 4 chunks
	var content string
	for i := 0; i < 200; i++ {
		content += "line of code\n"
	}
	repoDir := createTestRepo(t, map[string]string{
		"big.go": content,
	})

	stats, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}

	if stats.Indexed != 1 {
		t.Errorf("indexed: got %d, want 1", stats.Indexed)
	}

	// Only 1 embedding returned (maxFit=1), so only 1 stored
	// (the ci >= len(chunks) guard prevents out-of-bounds)
	records, _ := db.GetAllEmbeddings()
	if len(records) != 1 {
		t.Errorf("stored chunks: got %d, want 1 (short batch returns only 1)", len(records))
	}
}

func TestIndexer_Search_ZeroVectorsRankLow(t *testing.T) {
	// Zero-vector chunks should have ~0 cosine similarity with any query,
	// so they should rank at the bottom of search results.
	db := tempDB(t)
	emb := newZeroVectorBatchEmbedder(8, 2) // zero-vector every 2nd chunk
	indexer := NewIndexer(db, emb)

	var content string
	for i := 0; i < 150; i++ {
		content += fmt.Sprintf("func handler%d() { /* process request */ }\n", i)
	}
	repoDir := createTestRepo(t, map[string]string{
		"handlers.go": content,
	})

	_, err := indexer.IndexRepo(context.Background(), "repo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}

	results, err := indexer.Search(context.Background(), "handler", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	// The last result should have the lowest score (likely a zero-vector chunk)
	last := results[len(results)-1]
	first := results[0]
	if last.Score >= first.Score && len(results) > 1 {
		t.Errorf("expected last result score (%f) < first result score (%f)", last.Score, first.Score)
	}
}

// --- Search tests ---

func TestIndexer_Search_Basic(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	// Index some files with varying content lengths to get different embeddings
	repoDir := createTestRepo(t, map[string]string{
		"auth.go":    "package auth\n\nfunc Login() {}\nfunc Logout() {}\n",
		"handler.go": "package handler\n\nfunc HandleRequest() {}\n",
	})

	_, err := indexer.IndexRepo(context.Background(), "myrepo", repoDir)
	if err != nil {
		t.Fatalf("IndexRepo: %v", err)
	}

	results, err := indexer.Search(context.Background(), "authentication", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}

	// Results should be ranked by score (descending)
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: score[%d]=%f > score[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestIndexer_Search_EmptyIndex(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	results, err := indexer.Search(context.Background(), "anything", 10)
	if err != nil {
		t.Fatalf("Search on empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty index, got %d", len(results))
	}
}

func TestIndexer_Search_RespectsLimit(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	// Create many small files
	files := make(map[string]string)
	for i := 0; i < 20; i++ {
		name := filepath.Join("pkg", string(rune('a'+i))+".go")
		files[name] = "package pkg\n\nfunc Fn() {}\n"
	}
	repoDir := createTestRepo(t, files)

	indexer.IndexRepo(context.Background(), "repo", repoDir)

	results, err := indexer.Search(context.Background(), "function", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

func TestIndexer_Search_ResultFields(t *testing.T) {
	db := tempDB(t)
	emb := newMockEmbedder(8)
	indexer := NewIndexer(db, emb)

	repoDir := createTestRepo(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	indexer.IndexRepo(context.Background(), "myrepo", repoDir)

	results, err := indexer.Search(context.Background(), "main", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	r := results[0]
	if r.Repo != "myrepo" {
		t.Errorf("repo: got %q, want %q", r.Repo, "myrepo")
	}
	if r.File != "main.go" {
		t.Errorf("file: got %q, want %q", r.File, "main.go")
	}
	if r.LineStart < 1 {
		t.Errorf("line_start: got %d, want >= 1", r.LineStart)
	}
	if r.LineEnd < r.LineStart {
		t.Errorf("line_end (%d) < line_start (%d)", r.LineEnd, r.LineStart)
	}
	// With mock embeddings, score can be negative; just verify it's computed
	if r.Score == 0 {
		t.Errorf("score: got 0, want non-zero")
	}
}

// --- Mock embedder tests ---

func TestMockEmbedder_DeterministicOutput(t *testing.T) {
	emb := newMockEmbedder(4)

	v1, _ := emb.Embed(context.Background(), "hello")
	v2, _ := emb.Embed(context.Background(), "hello")

	if len(v1) != 4 || len(v2) != 4 {
		t.Fatal("expected 4-dimensional vectors")
	}

	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("dimension %d: %f != %f (not deterministic)", i, v1[i], v2[i])
		}
	}
}

func TestMockEmbedder_DifferentTextsDifferentVectors(t *testing.T) {
	emb := newMockEmbedder(4)

	v1, _ := emb.Embed(context.Background(), "short")      // len 5
	v2, _ := emb.Embed(context.Background(), "a longer text") // len 13

	same := true
	for i := range v1 {
		if v1[i] != v2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different vectors for different-length texts")
	}
}

func TestMockEmbedder_BatchMatchesSequential(t *testing.T) {
	emb := newMockEmbedder(4)

	texts := []string{"alpha", "beta", "gamma"}
	batch, err := emb.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(batch) != 3 {
		t.Fatalf("batch length: got %d, want 3", len(batch))
	}

	// Reset call count and verify individual calls match
	for i, text := range texts {
		single, _ := emb.Embed(context.Background(), text)
		for j := range single {
			if single[j] != batch[i][j] {
				t.Errorf("text %d, dim %d: batch=%f single=%f", i, j, batch[i][j], single[j])
			}
		}
	}
}
