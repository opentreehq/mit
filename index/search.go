package index

import (
	"math"
	"sort"
)

// SearchResult represents a search hit.
type SearchResult struct {
	Repo      string  `json:"repo"`
	File      string  `json:"file"`
	LineStart int     `json:"line_start"`
	LineEnd   int     `json:"line_end"`
	Score     float64 `json:"score"`
	Content   string  `json:"content,omitempty"`
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// RankResults sorts search results by score descending and returns top-k.
func RankResults(results []SearchResult, limit int) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// BytesToFloat32 converts a byte slice to a float32 slice.
// The byte slice must have length divisible by 4.
func BytesToFloat32(data []byte) []float32 {
	if len(data)%4 != 0 {
		return nil
	}
	result := make([]float32, len(data)/4)
	for i := range result {
		bits := uint32(data[i*4]) |
			uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 |
			uint32(data[i*4+3])<<24
		result[i] = math.Float32frombits(bits)
	}
	return result
}

// Float32ToBytes converts a float32 slice to a byte slice.
func Float32ToBytes(data []float32) []byte {
	result := make([]byte, len(data)*4)
	for i, f := range data {
		bits := math.Float32bits(f)
		result[i*4] = byte(bits)
		result[i*4+1] = byte(bits >> 8)
		result[i*4+2] = byte(bits >> 16)
		result[i*4+3] = byte(bits >> 24)
	}
	return result
}
