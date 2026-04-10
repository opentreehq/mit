#ifndef MIT_EMBEDDING_BINDING_H
#define MIT_EMBEDDING_BINDING_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

// Opaque handle to the embedding context.
typedef struct mit_embedder mit_embedder;

// Initialize the embedding model. Returns NULL on failure.
// n_threads: number of threads for inference (0 = auto).
// n_gpu_layers: layers to offload to GPU (-1 = all).
mit_embedder* mit_embedder_init(const char* model_path, int n_threads, int n_gpu_layers);

// Return the embedding dimension for the loaded model.
int mit_embedder_dimensions(mit_embedder* ctx);

// Embed a single text. Writes n_embd floats to out_embedding.
// out_embedding must be pre-allocated with at least mit_embedder_dimensions() floats.
// Returns 0 on success, non-zero on error.
int mit_embedder_embed(mit_embedder* ctx, const char* text, int text_len, float* out_embedding);

// Embed multiple texts in a single batch decode.
// texts: array of text pointers. text_lens: array of lengths. n: number of texts.
// out_embeddings: pre-allocated float array of size n * dimensions (row-major).
// Returns number of successfully embedded texts (may be < n if total tokens exceed context).
int mit_embedder_embed_batch(mit_embedder* ctx, const char** texts, const int* text_lens, int n, float* out_embeddings);

// Free resources.
void mit_embedder_free(mit_embedder* ctx);

#ifdef __cplusplus
}
#endif

#endif // MIT_EMBEDDING_BINDING_H
