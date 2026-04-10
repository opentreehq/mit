package llama_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabemeola/mit/embedding/llama"
)

func modelPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	p := filepath.Join(home, ".mit", "models", "Qwen3-Embedding-0.6B-Q8_0.gguf")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Skip("model not downloaded, run 'mit index' first to trigger download")
	}
	return p
}

func TestE2E_EmbedSingleText(t *testing.T) {
	p := modelPath(t)

	emb, err := llama.New(p, 0, -1)
	if err != nil {
		t.Fatalf("failed to init embedder: %v", err)
	}
	defer emb.Close()

	if emb.Dimensions() != 1024 {
		t.Fatalf("expected 1024 dimensions, got %d", emb.Dimensions())
	}

	vec, err := emb.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	if len(vec) != 1024 {
		t.Fatalf("expected 1024-dim vector, got %d", len(vec))
	}

	// Verify it's normalized (L2 norm ≈ 1.0)
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Fatalf("expected L2 norm ≈ 1.0, got %f", norm)
	}

	// Verify it's not all zeros
	allZero := true
	for _, v := range vec {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("embedding is all zeros")
	}
}

func TestE2E_SimilarTextsMoreSimilar(t *testing.T) {
	p := modelPath(t)

	emb, err := llama.New(p, 0, -1)
	if err != nil {
		t.Fatalf("failed to init embedder: %v", err)
	}
	defer emb.Close()

	ctx := context.Background()

	vecA, err := emb.Embed(ctx, "the cat sat on the mat")
	if err != nil {
		t.Fatalf("embed A: %v", err)
	}

	vecB, err := emb.Embed(ctx, "the kitten rested on the rug")
	if err != nil {
		t.Fatalf("embed B: %v", err)
	}

	vecC, err := emb.Embed(ctx, "quantum chromodynamics describes the strong nuclear force")
	if err != nil {
		t.Fatalf("embed C: %v", err)
	}

	simAB := cosine(vecA, vecB)
	simAC := cosine(vecA, vecC)

	t.Logf("sim(cat/kitten) = %.4f", simAB)
	t.Logf("sim(cat/quantum) = %.4f", simAC)

	if simAB <= simAC {
		t.Fatalf("expected similar texts to have higher similarity: sim(A,B)=%f <= sim(A,C)=%f", simAB, simAC)
	}
}

func TestE2E_EmbedBatch(t *testing.T) {
	p := modelPath(t)

	emb, err := llama.New(p, 0, -1)
	if err != nil {
		t.Fatalf("failed to init embedder: %v", err)
	}
	defer emb.Close()

	texts := []string{
		"func main() { fmt.Println(\"hello\") }",
		"import numpy as np",
		"SELECT * FROM users WHERE id = 1",
	}

	vecs, err := emb.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("batch embed failed: %v", err)
	}

	if len(vecs) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(vecs))
	}

	for i, v := range vecs {
		if len(v) != 1024 {
			t.Fatalf("vector %d: expected 1024 dims, got %d", i, len(v))
		}
	}
}

func TestE2E_CodeSearchRelevance(t *testing.T) {
	p := modelPath(t)

	emb, err := llama.New(p, 0, -1)
	if err != nil {
		t.Fatalf("failed to init embedder: %v", err)
	}
	defer emb.Close()

	ctx := context.Background()

	query, _ := emb.Embed(ctx, "HTTP request handler")
	code1, _ := emb.Embed(ctx, "func handleRequest(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }")
	code2, _ := emb.Embed(ctx, "func calculateTax(amount float64, rate float64) float64 { return amount * rate }")

	simQuery1 := cosine(query, code1)
	simQuery2 := cosine(query, code2)

	t.Logf("sim(query, http_handler) = %.4f", simQuery1)
	t.Logf("sim(query, tax_calc)     = %.4f", simQuery2)

	if simQuery1 <= simQuery2 {
		t.Fatalf("expected HTTP handler to be more relevant: sim1=%f <= sim2=%f", simQuery1, simQuery2)
	}
}

func cosine(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
