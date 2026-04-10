package index

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabemeola/mit/config"
)

const (
	DefaultChunkSize = 50  // lines per chunk
	MaxChunkSize     = 100 // absolute max lines per chunk
	MaxChunkBytes    = 24000 // ~6000 tokens at ~4 bytes/token; well under 8192 token context
)

// Chunk represents a piece of a source file.
type Chunk struct {
	Repo      string `json:"repo"`
	File      string `json:"file"`
	Index     int    `json:"index"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Content   string `json:"content"`
}

// ChunkFile splits a file into chunks of roughly chunkSize lines,
// also respecting a byte budget (MaxChunkBytes) so chunks fit within
// the embedding model's token context window.
func ChunkFile(repo, filePath string, chunkSize int) ([]Chunk, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var chunks []Chunk
	var current strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	lineNum := 0
	chunkStart := 1
	chunkIndex := 0

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, Chunk{
				Repo:      repo,
				File:      filePath,
				Index:     chunkIndex,
				LineStart: chunkStart,
				LineEnd:   lineNum,
				Content:   current.String(),
			})
			current.Reset()
			chunkStart = lineNum + 1
			chunkIndex++
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Check if adding this line would exceed byte budget
		newBytes := current.Len() + len(line) + 1 // +1 for newline
		if current.Len() > 0 && newBytes > MaxChunkBytes {
			// Flush current chunk before adding this line
			lineNum-- // don't count this line yet
			flush()
			lineNum++ // restore
			chunkStart = lineNum
		}

		current.WriteString(line)
		current.WriteString("\n")

		if lineNum-chunkStart+1 >= chunkSize {
			flush()
		}
	}

	// Remaining lines
	flush()

	return chunks, scanner.Err()
}

// ShouldIndex returns true if the file should be indexed.
func ShouldIndex(path string) bool {
	if IsMinified(path) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".rb", ".rs",
		".java", ".kt", ".swift", ".c", ".cpp", ".h", ".hpp",
		".cs", ".php", ".sh", ".bash", ".zsh", ".fish",
		".yaml", ".yml", ".json", ".toml", ".xml",
		".md", ".txt", ".sql", ".graphql", ".proto",
		".html", ".css", ".scss", ".less", ".vue", ".svelte":
		return true
	}
	return false
}

// IsMinified returns true if the file appears to be minified/bundled,
// based on filename patterns or content sampling (avg line length > 500).
func IsMinified(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	// Check common minified file patterns
	if strings.HasSuffix(base, ".min.js") ||
		strings.HasSuffix(base, ".min.css") ||
		strings.HasSuffix(base, ".min.mjs") ||
		strings.HasSuffix(base, ".bundle.js") ||
		strings.HasSuffix(base, ".bundle.css") {
		return true
	}

	// Content-based heuristic: sample first few lines
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" || ext == ".css" || ext == ".mjs" {
		if avgLineLength(path, 5) > 500 {
			return true
		}
	}
	return false
}

// avgLineLength returns the average line length of the first n lines.
func avgLineLength(path string, n int) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	total := 0
	count := 0
	for scanner.Scan() && count < n {
		total += len(scanner.Text())
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

// DefaultIgnoreDirs are directories and files skipped when no ignore set is provided.
// This is the fallback when no config is loaded; the config's DefaultIndexIgnore is authoritative.
var DefaultIgnoreDirs = map[string]bool{
	".git": true, ".sl": true, "node_modules": true, "vendor": true,
	".next": true, "dist": true, "build": true, "__pycache__": true,
	".cache": true, "target": true,
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"go.sum": true, "Cargo.lock": true,
	".env": true,
}

func init() {
	DefaultIgnoreDirs[config.DataDir] = true
	DefaultIgnoreDirs[config.DataDir+"-worktrees"] = true
}

// WalkResult contains the results of walking a repo directory.
type WalkResult struct {
	Files           []string
	SkippedMinified []string
	SkippedDirs     []string
}

// WalkRepo walks a repo directory and returns all indexable files.
// If ignoreDirs is nil, DefaultIgnoreDirs is used.
func WalkRepo(repoPath string, ignoreDirs ...map[string]bool) ([]string, error) {
	result, err := WalkRepoDetailed(repoPath, ignoreDirs...)
	return result.Files, err
}

// WalkRepoDetailed walks a repo directory and returns detailed results including skips.
func WalkRepoDetailed(repoPath string, ignoreDirs ...map[string]bool) (WalkResult, error) {
	ignore := DefaultIgnoreDirs
	if len(ignoreDirs) > 0 && ignoreDirs[0] != nil {
		ignore = ignoreDirs[0]
	}

	var result WalkResult
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			if ignore[filepath.Base(path)] {
				rel, _ := filepath.Rel(repoPath, path)
				result.SkippedDirs = append(result.SkippedDirs, rel)
				return filepath.SkipDir
			}
			return nil
		}
		// Check if file name matches ignore list (e.g. package-lock.json)
		base := filepath.Base(path)
		if ignore[base] {
			return nil
		}
		// Skip .env variants (.env.local, .env.production, etc.)
		if strings.HasPrefix(base, ".env") {
			return nil
		}
		if IsMinified(path) {
			rel, _ := filepath.Rel(repoPath, path)
			result.SkippedMinified = append(result.SkippedMinified, rel)
			return nil
		}
		if ShouldIndex(path) {
			result.Files = append(result.Files, path)
		}
		return nil
	})
	return result, err
}
