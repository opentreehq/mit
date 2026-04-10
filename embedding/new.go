//go:build !noembed

package embedding

import "github.com/opentreehq/mit/embedding/llama"

// NewEmbedder creates a LlamaEmbedder backed by llama.cpp.
// modelPath: path to GGUF model file.
// nThreads: inference threads (0 = auto).
// nGPULayers: layers to offload (-1 = all).
func NewEmbedder(modelPath string, nThreads, nGPULayers int) (Embedder, error) {
	return llama.New(modelPath, nThreads, nGPULayers)
}
