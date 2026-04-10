#include "binding.h"
#include "llama.h"

#include <cmath>
#include <cstring>
#include <vector>

struct mit_embedder {
    llama_model* model;
    llama_context* ctx;
    const llama_vocab* vocab;
    int n_embd;
    enum llama_pooling_type pooling_type;
};

static void normalize(const float* in, float* out, int n) {
    double sum = 0.0;
    for (int i = 0; i < n; i++) {
        sum += (double)in[i] * (double)in[i];
    }
    double norm = sqrt(sum);
    if (norm < 1e-12) {
        norm = 1e-12;
    }
    for (int i = 0; i < n; i++) {
        out[i] = (float)((double)in[i] / norm);
    }
}

// Suppress all llama.cpp log output
static void silent_log(enum ggml_log_level level, const char* text, void* user_data) {
    (void)level;
    (void)text;
    (void)user_data;
}

mit_embedder* mit_embedder_init(const char* model_path, int n_threads, int n_gpu_layers) {
    // Suppress log spam before anything else
    llama_log_set(silent_log, nullptr);

    llama_backend_init();

    // Load model
    llama_model_params model_params = llama_model_default_params();
    model_params.n_gpu_layers = n_gpu_layers;

    llama_model* model = llama_model_load_from_file(model_path, model_params);
    if (!model) {
        return nullptr;
    }

    // Create context - let the model choose its own pooling type
    llama_context_params ctx_params = llama_context_default_params();
    ctx_params.n_ctx = 8192;
    ctx_params.n_batch = 8192;
    ctx_params.n_ubatch = 8192;
    ctx_params.n_seq_max = 64;  // support batch embedding of up to 64 sequences
    ctx_params.embeddings = true;
    ctx_params.n_threads = n_threads > 0 ? n_threads : 4;
    ctx_params.n_threads_batch = ctx_params.n_threads;
    ctx_params.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED;  // use model default

    llama_context* ctx = llama_init_from_model(model, ctx_params);
    if (!ctx) {
        llama_model_free(model);
        return nullptr;
    }

    mit_embedder* embedder = new mit_embedder();
    embedder->model = model;
    embedder->ctx = ctx;
    embedder->vocab = llama_model_get_vocab(model);
    embedder->n_embd = llama_model_n_embd(model);
    embedder->pooling_type = llama_pooling_type(ctx);

    return embedder;
}

int mit_embedder_dimensions(mit_embedder* ctx) {
    if (!ctx) return 0;
    return ctx->n_embd;
}

int mit_embedder_embed(mit_embedder* ctx, const char* text, int text_len, float* out_embedding) {
    if (!ctx || !text || !out_embedding) return -1;

    // Tokenize
    int n_tokens_max = text_len + 128;
    std::vector<llama_token> tokens(n_tokens_max);

    int n_tokens = llama_tokenize(ctx->vocab, text, text_len, tokens.data(), n_tokens_max, true, true);
    if (n_tokens < 0) {
        n_tokens_max = -n_tokens;
        tokens.resize(n_tokens_max);
        n_tokens = llama_tokenize(ctx->vocab, text, text_len, tokens.data(), n_tokens_max, true, true);
        if (n_tokens < 0) {
            return -2;
        }
    }

    // Truncate to context size to avoid GGML_ASSERT crash
    const int n_ctx = 8192;
    if (n_tokens > n_ctx) {
        n_tokens = n_ctx;
    }

    // Build batch - mark all tokens as output (needed for pooling)
    llama_batch batch = llama_batch_init(n_tokens, 0, 1);
    for (int i = 0; i < n_tokens; i++) {
        batch.token[i] = tokens[i];
        batch.pos[i] = i;
        batch.n_seq_id[i] = 1;
        batch.seq_id[i][0] = 0;
        batch.logits[i] = 1;  // mark all as output for pooling
    }
    batch.n_tokens = n_tokens;

    // Clear KV cache if present (encode-only contexts may not have one)
    auto mem = llama_get_memory(ctx->ctx);
    if (mem) {
        llama_memory_clear(mem, true);
    }

    // Decode (llama.cpp uses decode even for embeddings when pooling is active)
    int ret = llama_decode(ctx->ctx, batch);
    llama_batch_free(batch);

    if (ret != 0) {
        return -3;
    }

    // Get pooled sequence embedding
    const float* embd = llama_get_embeddings_seq(ctx->ctx, 0);
    if (!embd) {
        // Fallback to last token embedding
        embd = llama_get_embeddings_ith(ctx->ctx, n_tokens - 1);
        if (!embd) {
            return -4;
        }
    }

    // Normalize (L2)
    normalize(embd, out_embedding, ctx->n_embd);

    return 0;
}

int mit_embedder_embed_batch(mit_embedder* ctx, const char** texts, const int* text_lens, int n, float* out_embeddings) {
    if (!ctx || !texts || !text_lens || !out_embeddings || n <= 0) return 0;

    const int n_ctx = 8192;

    // Tokenize all texts and figure out how many fit in the context window
    struct SeqInfo {
        std::vector<llama_token> tokens;
    };
    std::vector<SeqInfo> seqs(n);

    int total_tokens = 0;
    int n_fit = 0;

    for (int s = 0; s < n; s++) {
        int max_tok = text_lens[s] + 128;
        seqs[s].tokens.resize(max_tok);

        int nt = llama_tokenize(ctx->vocab, texts[s], text_lens[s],
                                seqs[s].tokens.data(), max_tok, true, true);
        if (nt < 0) {
            max_tok = -nt;
            seqs[s].tokens.resize(max_tok);
            nt = llama_tokenize(ctx->vocab, texts[s], text_lens[s],
                                seqs[s].tokens.data(), max_tok, true, true);
            if (nt < 0) {
                break; // can't tokenize, stop here
            }
        }

        // Truncate individual sequence if needed
        if (nt > n_ctx) nt = n_ctx;

        // Check if adding this sequence would exceed context
        if (total_tokens + nt > n_ctx) {
            break; // can't fit more sequences
        }

        seqs[s].tokens.resize(nt);
        total_tokens += nt;
        n_fit = s + 1;
    }

    if (n_fit == 0) return 0;

    // Build batch with all sequences, each with a unique seq_id
    llama_batch batch = llama_batch_init(total_tokens, 0, n_fit);
    int pos = 0;
    for (int s = 0; s < n_fit; s++) {
        for (int i = 0; i < (int)seqs[s].tokens.size(); i++) {
            int idx = pos + i;
            batch.token[idx] = seqs[s].tokens[i];
            batch.pos[idx] = i;  // position within this sequence
            batch.n_seq_id[idx] = 1;
            batch.seq_id[idx][0] = s;  // unique seq_id per text
            batch.logits[idx] = 1;     // mark for pooling
        }
        pos += (int)seqs[s].tokens.size();
    }
    batch.n_tokens = total_tokens;

    // Clear memory
    auto mem = llama_get_memory(ctx->ctx);
    if (mem) {
        llama_memory_clear(mem, true);
    }

    // Single decode for all sequences
    int ret = llama_decode(ctx->ctx, batch);
    llama_batch_free(batch);

    if (ret != 0) return -1;  // decode failed

    // Extract per-sequence pooled embeddings
    for (int s = 0; s < n_fit; s++) {
        float* out = out_embeddings + s * ctx->n_embd;
        const float* embd = llama_get_embeddings_seq(ctx->ctx, s);
        if (!embd) {
            // Fallback: zero out this embedding
            memset(out, 0, ctx->n_embd * sizeof(float));
            continue;
        }
        normalize(embd, out, ctx->n_embd);
    }

    return n_fit;
}

void mit_embedder_free(mit_embedder* ctx) {
    if (!ctx) return;
    if (ctx->ctx) llama_free(ctx->ctx);
    if (ctx->model) llama_model_free(ctx->model);
    delete ctx;
    llama_backend_free();
}
