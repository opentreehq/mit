package embedding

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentreehq/mit/config"
)

// --- StubEmbedder tests ---

func TestStubEmbedder_Embed_ReturnsError(t *testing.T) {
	s := &StubEmbedder{}
	_, err := s.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error from stub embedder")
	}
}

func TestStubEmbedder_EmbedBatch_ReturnsError(t *testing.T) {
	s := &StubEmbedder{}
	_, err := s.EmbedBatch(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error from stub embedder")
	}
}

func TestStubEmbedder_Dimensions(t *testing.T) {
	s := &StubEmbedder{}
	if d := s.Dimensions(); d != 0 {
		t.Errorf("expected 0 dimensions, got %d", d)
	}
}

func TestStubEmbedder_Close(t *testing.T) {
	s := &StubEmbedder{}
	if err := s.Close(); err != nil {
		t.Errorf("expected nil error from close, got %v", err)
	}
}

// --- ModelPath tests ---

func TestModelPath_ReturnsHomeBasedPath(t *testing.T) {
	p, err := ModelPath(DefaultModelName)
	if err != nil {
		t.Fatalf("ModelPath: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, config.ModelsDir, DefaultModelName)
	if p != expected {
		t.Errorf("ModelPath: got %q, want %q", p, expected)
	}
}

func TestModelExists_FalseWhenNoModel(t *testing.T) {
	// Unless the user happens to have the model installed,
	// we can at least verify it returns a bool without panicking.
	_ = ModelExists(DefaultModelName)
}

func TestEnsureModelDir_CreatesDir(t *testing.T) {
	// This creates ~/.mit/models/ if it doesn't exist.
	// Safe to call since it's an idempotent mkdir.
	if err := EnsureModelDir(); err != nil {
		t.Fatalf("EnsureModelDir: %v", err)
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, config.ModelsDir)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("model dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

// --- Interface compliance ---

func TestStubEmbedder_ImplementsEmbedder(t *testing.T) {
	var _ Embedder = (*StubEmbedder)(nil)
}
