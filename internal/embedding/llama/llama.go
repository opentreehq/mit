// Package llama provides embedding via llama.cpp CGo bindings.
package llama

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/llama.cpp/include -I${SRCDIR}/../../../third_party/llama.cpp/ggml/include
#cgo CXXFLAGS: -I${SRCDIR}/../../../third_party/llama.cpp/include -I${SRCDIR}/../../../third_party/llama.cpp/ggml/include -std=c++17
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/llama.cpp/build/src -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-metal -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-blas -L${SRCDIR}/../../../third_party/llama.cpp/build/ggml/src/ggml-cpu
#cgo LDFLAGS: -lllama -lggml -lggml-base -lggml-cpu
#cgo !darwin LDFLAGS: -lstdc++
#cgo darwin LDFLAGS: -lggml-metal -lggml-blas -framework Foundation -framework Metal -framework MetalKit -framework Accelerate
#include <stdlib.h>
#include "binding.h"
*/
import "C"
import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Embedder implements embedding.Embedder using llama.cpp via CGo.
type Embedder struct {
	ctx  *C.mit_embedder
	dims int
	mu   sync.Mutex
}

// New creates a new embedder backed by llama.cpp.
// modelPath: path to GGUF model file.
// nThreads: inference threads (0 = number of CPUs).
// nGPULayers: layers to offload (-1 = all).
func New(modelPath string, nThreads, nGPULayers int) (*Embedder, error) {
	if nThreads <= 0 {
		nThreads = runtime.NumCPU()
	}

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	ctx := C.mit_embedder_init(cPath, C.int(nThreads), C.int(nGPULayers))
	if ctx == nil {
		return nil, fmt.Errorf("failed to load embedding model: %s", modelPath)
	}

	dims := int(C.mit_embedder_dimensions(ctx))

	return &Embedder{
		ctx:  ctx,
		dims: dims,
	}, nil
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.ctx == nil {
		return nil, fmt.Errorf("embedder is closed")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	out := make([]float32, e.dims)

	ret := C.mit_embedder_embed(e.ctx, cText, C.int(len(text)), (*C.float)(unsafe.Pointer(&out[0])))
	if ret != 0 {
		return nil, fmt.Errorf("embedding failed (code %d)", int(ret))
	}

	return out, nil
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) == 1 {
		emb, err := e.Embed(ctx, texts[0])
		if err != nil {
			// Single text failed — return zero vector instead of error
			return [][]float32{make([]float32, e.dims)}, nil
		}
		return [][]float32{emb}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx == nil {
		return nil, fmt.Errorf("embedder is closed")
	}

	// Build C arrays
	n := len(texts)
	cTexts := make([]*C.char, n)
	cLens := make([]C.int, n)
	for i, t := range texts {
		cTexts[i] = C.CString(t)
		cLens[i] = C.int(len(t))
	}
	defer func() {
		for _, ct := range cTexts {
			C.free(unsafe.Pointer(ct))
		}
	}()

	// Flat output buffer: n * dims
	outFlat := make([]float32, n*e.dims)

	nEmbedded := int(C.mit_embedder_embed_batch(
		e.ctx,
		(**C.char)(unsafe.Pointer(&cTexts[0])),
		(*C.int)(unsafe.Pointer(&cLens[0])),
		C.int(n),
		(*C.float)(unsafe.Pointer(&outFlat[0])),
	))

	if nEmbedded <= 0 {
		// Batch decode failed — fall back to individual embedding for all texts.
		// Each individual embed clears the context memory, recovering from
		// whatever state the failed batch left behind.
		results := make([][]float32, 0, n)
		for i := 0; i < n; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			emb, err := e.embedUnlocked(ctx, texts[i])
			if err != nil {
				emb = make([]float32, e.dims)
			}
			results = append(results, emb)
		}
		return results, nil
	}

	// Split flat buffer into per-text embeddings
	results := make([][]float32, nEmbedded)
	for i := 0; i < nEmbedded; i++ {
		results[i] = outFlat[i*e.dims : (i+1)*e.dims]
	}

	// If some texts didn't fit in the batch, embed them individually
	if nEmbedded < n {
		for i := nEmbedded; i < n; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			emb, err := e.embedUnlocked(ctx, texts[i])
			if err != nil {
				emb = make([]float32, e.dims)
			}
			results = append(results, emb)
		}
	}

	return results, nil
}

// embedUnlocked is Embed without acquiring the mutex (caller must hold it).
func (e *Embedder) embedUnlocked(ctx context.Context, text string) ([]float32, error) {
	if e.ctx == nil {
		return nil, fmt.Errorf("embedder is closed")
	}

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	out := make([]float32, e.dims)

	ret := C.mit_embedder_embed(e.ctx, cText, C.int(len(text)), (*C.float)(unsafe.Pointer(&out[0])))
	if ret != 0 {
		return nil, fmt.Errorf("embedding failed (code %d)", int(ret))
	}

	return out, nil
}

func (e *Embedder) Dimensions() int {
	return e.dims
}

func (e *Embedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx != nil {
		C.mit_embedder_free(e.ctx)
		e.ctx = nil
	}
	return nil
}
