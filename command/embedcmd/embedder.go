package embedcmd

import (
	"fmt"
	"os"

	"github.com/gabemeola/mit/config"
	"github.com/gabemeola/mit/embedding"
)

func modelSpecFromConfig(cfg *config.Config) embedding.ModelSpec {
	if cfg != nil && cfg.Index.Model != nil && cfg.Index.Model.URL != "" {
		return embedding.ModelSpec{
			URL: cfg.Index.Model.URL,
		}
	}
	return embedding.DefaultModel()
}

func loadEmbedder(cfg *config.Config, quiet bool) (embedding.Embedder, error) {
	spec := modelSpecFromConfig(cfg)

	needsDownload := !embedding.ModelExists(spec.ResolveName())

	if needsDownload && spec.URL == "" {
		return nil, fmt.Errorf("model %q not found and no download URL configured", spec.ResolveName())
	}

	modelPath, err := embedding.EnsureModel(spec, func(downloaded, total int64) {
		if quiet {
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

	if needsDownload && !quiet {
		fmt.Fprintln(os.Stderr)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Loading model %s...", spec.ResolveName())
	}

	emb, err := embedding.NewEmbedder(modelPath, 0, -1)
	if err != nil {
		if !quiet {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return nil, fmt.Errorf("loading embedding model: %w", err)
	}

	if !quiet {
		fmt.Fprintln(os.Stderr, " ready")
	}

	return emb, nil
}
