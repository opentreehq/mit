package index

import (
	"math"
	"testing"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 2, 3}
	score := CosineSimilarity(a, a)
	if math.Abs(score-1.0) > 0.0001 {
		t.Errorf("identical vectors should have similarity 1.0, got %f", score)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score := CosineSimilarity(a, b)
	if math.Abs(score) > 0.0001 {
		t.Errorf("orthogonal vectors should have similarity 0.0, got %f", score)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	score := CosineSimilarity(a, b)
	if math.Abs(score-(-1.0)) > 0.0001 {
		t.Errorf("opposite vectors should have similarity -1.0, got %f", score)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	score := CosineSimilarity(nil, nil)
	if score != 0 {
		t.Errorf("empty vectors should have similarity 0, got %f", score)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("different length vectors should have similarity 0, got %f", score)
	}
}

func TestRankResults(t *testing.T) {
	results := []SearchResult{
		{Score: 0.5, File: "a.go"},
		{Score: 0.9, File: "b.go"},
		{Score: 0.3, File: "c.go"},
		{Score: 0.7, File: "d.go"},
	}

	ranked := RankResults(results, 2)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 results, got %d", len(ranked))
	}
	if ranked[0].File != "b.go" {
		t.Errorf("expected first result 'b.go', got %q", ranked[0].File)
	}
	if ranked[1].File != "d.go" {
		t.Errorf("expected second result 'd.go', got %q", ranked[1].File)
	}
}

func TestRankResults_NoLimit(t *testing.T) {
	results := []SearchResult{
		{Score: 0.5}, {Score: 0.9}, {Score: 0.3},
	}
	ranked := RankResults(results, 0)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 results with limit 0, got %d", len(ranked))
	}
}

func TestFloat32RoundTrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 3.14159}
	bytes := Float32ToBytes(original)
	recovered := BytesToFloat32(bytes)

	if len(recovered) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(recovered), len(original))
	}
	for i := range original {
		if original[i] != recovered[i] {
			t.Errorf("index %d: expected %f, got %f", i, original[i], recovered[i])
		}
	}
}

func TestBytesToFloat32_InvalidLength(t *testing.T) {
	result := BytesToFloat32([]byte{1, 2, 3}) // not divisible by 4
	if result != nil {
		t.Error("expected nil for invalid byte length")
	}
}
