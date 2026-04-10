package embedding

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const (
	ModelsDir        = ".mit/models"
	DefaultModelName = "Qwen3-Embedding-0.6B-Q8_0.gguf"
	DefaultModelURL  = "https://huggingface.co/Qwen/Qwen3-Embedding-0.6B-GGUF/resolve/main/Qwen3-Embedding-0.6B-Q8_0.gguf"
)

// ModelSpec describes which model to use.
type ModelSpec struct {
	Name string // local filename; derived from URL if empty
	URL  string
}

// ResolveName returns the model filename, deriving it from URL if Name is empty.
func (s ModelSpec) ResolveName() string {
	if s.Name != "" {
		return s.Name
	}
	if s.URL != "" {
		if u, err := url.Parse(s.URL); err == nil {
			return filepath.Base(u.Path)
		}
	}
	return DefaultModelName
}

// DefaultModel returns the default embedding model spec.
func DefaultModel() ModelSpec {
	return ModelSpec{Name: DefaultModelName, URL: DefaultModelURL}
}

// ModelPath returns the full path to the model file.
func ModelPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ModelsDir, name), nil
}

// ModelExists checks if a model file exists.
func ModelExists(name string) bool {
	path, err := ModelPath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// EnsureModelDir creates the model directory if needed.
func EnsureModelDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ModelsDir)
	return os.MkdirAll(dir, 0755)
}

// DownloadModel downloads a model to ~/.mit/models/.
func DownloadModel(spec ModelSpec, progress func(downloaded, total int64)) error {
	if err := EnsureModelDir(); err != nil {
		return fmt.Errorf("creating model dir: %w", err)
	}

	modelPath, err := ModelPath(spec.ResolveName())
	if err != nil {
		return err
	}

	resp, err := http.Get(spec.URL)
	if err != nil {
		return fmt.Errorf("downloading model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmpPath := modelPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath) // clean up on failure
	}()

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	}

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("writing model: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing model file: %w", err)
	}

	if err := os.Rename(tmpPath, modelPath); err != nil {
		return fmt.Errorf("moving model file: %w", err)
	}

	return nil
}

// EnsureModel downloads the model if it doesn't exist. Returns the local path.
func EnsureModel(spec ModelSpec, progress func(downloaded, total int64)) (string, error) {
	modelPath, err := ModelPath(spec.ResolveName())
	if err != nil {
		return "", err
	}

	if ModelExists(spec.ResolveName()) {
		return modelPath, nil
	}

	if err := DownloadModel(spec, progress); err != nil {
		return "", err
	}

	return modelPath, nil
}

type progressReader struct {
	r          io.Reader
	total      int64
	downloaded int64
	fn         func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.downloaded += int64(n)
	pr.fn(pr.downloaded, pr.total)
	return n, err
}
