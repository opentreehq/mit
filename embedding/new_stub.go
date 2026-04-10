//go:build noembed

package embedding

import "fmt"

// NewEmbedder returns an error when built without embedding support.
func NewEmbedder(modelPath string, nThreads, nGPULayers int) (Embedder, error) {
	return nil, fmt.Errorf("embedding not available: build with CGo and llama.cpp support (remove -tags noembed)")
}
