package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	// Create a file with 250 lines
	var content strings.Builder
	for i := 1; i <= 250; i++ {
		content.WriteString("// line\n")
	}
	os.WriteFile(path, []byte(content.String()), 0644)

	chunks, err := ChunkFile("test-repo", path, 50)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	if len(chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(chunks))
	}

	// First chunk: lines 1-50
	if chunks[0].LineStart != 1 || chunks[0].LineEnd != 50 {
		t.Errorf("chunk 0: expected lines 1-50, got %d-%d", chunks[0].LineStart, chunks[0].LineEnd)
	}
	if chunks[0].Index != 0 {
		t.Errorf("chunk 0: expected index 0, got %d", chunks[0].Index)
	}

	// Second chunk: lines 51-100
	if chunks[1].LineStart != 51 || chunks[1].LineEnd != 100 {
		t.Errorf("chunk 1: expected lines 51-100, got %d-%d", chunks[1].LineStart, chunks[1].LineEnd)
	}

	// Last chunk: lines 201-250
	if chunks[4].LineStart != 201 || chunks[4].LineEnd != 250 {
		t.Errorf("chunk 4: expected lines 201-250, got %d-%d", chunks[4].LineStart, chunks[4].LineEnd)
	}

	for _, c := range chunks {
		if c.Repo != "test-repo" {
			t.Errorf("expected repo 'test-repo', got %q", c.Repo)
		}
	}
}

func TestChunkFile_ByteBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.json")

	// Create a file with long lines that exceed MaxChunkBytes before hitting line limit.
	// Each line is ~1000 bytes. At MaxChunkBytes=24000, should split around 24 lines.
	var content strings.Builder
	longLine := strings.Repeat("x", 999) // 999 chars + newline = 1000 bytes
	for i := 0; i < 100; i++ {
		content.WriteString(longLine)
		content.WriteString("\n")
	}
	os.WriteFile(path, []byte(content.String()), 0644)

	chunks, err := ChunkFile("repo", path, DefaultChunkSize)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}

	// With 1000 bytes/line and MaxChunkBytes=24000, each chunk should have ~24 lines
	// 100 lines / ~24 per chunk ≈ 4-5 chunks
	if len(chunks) < 4 {
		t.Errorf("expected at least 4 chunks for byte-budget splitting, got %d", len(chunks))
	}

	// Verify no chunk exceeds MaxChunkBytes (with some slack for the last partial line)
	for i, c := range chunks {
		if len(c.Content) > MaxChunkBytes+1000 { // +1000 for last line that triggers flush
			t.Errorf("chunk %d: %d bytes exceeds MaxChunkBytes (%d)", i, len(c.Content), MaxChunkBytes)
		}
	}

	// All lines should be accounted for
	totalLines := 0
	for _, c := range chunks {
		totalLines += c.LineEnd - c.LineStart + 1
	}
	if totalLines != 100 {
		t.Errorf("expected 100 total lines, got %d", totalLines)
	}
}

func TestChunkFile_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0644)

	chunks, err := ChunkFile("repo", path, 100)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestChunkFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	os.WriteFile(path, []byte(""), 0644)

	chunks, err := ChunkFile("repo", path, 100)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty file, got %d", len(chunks))
	}
}

func TestShouldIndex(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"index.ts", true},
		{"app.py", true},
		{"Cargo.toml", true},
		{"README.md", true},
		{"query.sql", true},
		{"image.png", false},
		{"binary.exe", false},
		{"archive.tar.gz", false},
		{"data.csv", false},
	}
	for _, tt := range tests {
		if got := ShouldIndex(tt.path); got != tt.want {
			t.Errorf("ShouldIndex(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestWalkRepo(t *testing.T) {
	dir := t.TempDir()

	// Create some files
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.MkdirAll(filepath.Join(dir, "node_modules", "dep"), 0755)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "lib.ts"), []byte("export {}"), 0644)
	os.WriteFile(filepath.Join(dir, "image.png"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "node_modules", "dep", "index.js"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0644)

	files, err := WalkRepo(dir)
	if err != nil {
		t.Fatalf("WalkRepo: %v", err)
	}

	// Should find main.go and src/lib.ts, skip node_modules and .git
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestWalkRepo_SkipsEnvFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=hunter2"), 0644)
	os.WriteFile(filepath.Join(dir, ".env.local"), []byte("DB_PASS=foo"), 0644)
	os.WriteFile(filepath.Join(dir, ".env.production"), []byte("API_KEY=bar"), 0644)
	os.WriteFile(filepath.Join(dir, ".env.development"), []byte("DEBUG=true"), 0644)

	files, err := WalkRepo(dir)
	if err != nil {
		t.Fatalf("WalkRepo: %v", err)
	}

	// Should only find main.go — all .env* files should be skipped
	if len(files) != 1 {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = filepath.Base(f)
		}
		t.Errorf("expected 1 file (main.go), got %d: %v", len(files), names)
	}
}
