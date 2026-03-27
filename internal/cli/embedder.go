//go:build !noembed

package cli

import (
	"fmt"
	"os"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/embedding"
)

// modelSpecFromConfig returns the model spec from config, or the default.
func modelSpecFromConfig(cfg *config.Config) embedding.ModelSpec {
	if cfg != nil && cfg.Index.Model != nil && cfg.Index.Model.URL != "" {
		return embedding.ModelSpec{
			URL: cfg.Index.Model.URL,
		}
	}
	return embedding.DefaultModel()
}

// loadEmbedder ensures the model is downloaded and returns a ready embedder.
func loadEmbedder(cfg *config.Config) (embedding.Embedder, error) {
	spec := modelSpecFromConfig(cfg)

	needsDownload := !embedding.ModelExists(spec.ResolveName())

	if needsDownload && spec.URL == "" {
		return nil, fmt.Errorf("model %q not found and no download URL configured", spec.ResolveName())
	}

	modelPath, err := embedding.EnsureModel(spec, func(downloaded, total int64) {
		if flagQuiet {
			return
		}
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			fmt.Fprintf(os.Stderr, "\rDownloading %s... %.0f%% (%d/%d MB)",
				spec.ResolveName(), pct, downloaded/1024/1024, total/1024/1024)
		} else {
			fmt.Fprintf(os.Stderr, "\rDownloading %s... %d MB",
				spec.ResolveName(), downloaded/1024/1024)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring embedding model: %w", err)
	}

	if needsDownload && !flagQuiet {
		fmt.Fprintln(os.Stderr)
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Loading model %s...", spec.ResolveName())
	}

	emb, err := embedding.NewEmbedder(modelPath, 0, -1)
	if err != nil {
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return nil, fmt.Errorf("loading embedding model: %w", err)
	}

	if !flagQuiet {
		fmt.Fprintln(os.Stderr, " ready")
	}

	return emb, nil
}
