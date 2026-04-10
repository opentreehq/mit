package embedding

import (
	"context"
	"fmt"
)

// StubEmbedder is a placeholder embedder that returns an error.
// Used when llama.cpp is not available (noembed build tag).
type StubEmbedder struct{}

func NewStubEmbedder() *StubEmbedder {
	return &StubEmbedder{}
}

func (s *StubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("embedding not available: build with CGo and llama.cpp support")
}

func (s *StubEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedding not available: build with CGo and llama.cpp support")
}

func (s *StubEmbedder) Dimensions() int {
	return 0
}

func (s *StubEmbedder) Close() error {
	return nil
}
