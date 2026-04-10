package index

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabemeola/mit/embedding"
	"github.com/gabemeola/mit/statedb"
)

// ProgressFunc is called during indexing to report progress.
// current is the file number being processed, total is the total file count,
// file is the relative path of the current file.
type ProgressFunc func(current, total int, file string)

// IndexStats tracks what happened during indexing.
type IndexStats struct {
	Indexed         int
	Unchanged       int
	SkippedMinified []string // files skipped because they are minified
}

// Indexer builds and maintains the semantic index.
type Indexer struct {
	db         *statedb.DB
	embedder   embedding.Embedder
	progress   ProgressFunc
	ignoreDirs map[string]bool
}

// NewIndexer creates a new indexer.
func NewIndexer(db *statedb.DB, embedder embedding.Embedder) *Indexer {
	return &Indexer{db: db, embedder: embedder}
}

// SetProgress sets the progress callback.
func (idx *Indexer) SetProgress(fn ProgressFunc) {
	idx.progress = fn
}

// SetIgnoreDirs sets the directories to skip during indexing.
func (idx *Indexer) SetIgnoreDirs(dirs map[string]bool) {
	idx.ignoreDirs = dirs
}

// IndexRepo indexes all files in a repo, skipping unchanged files.
func (idx *Indexer) IndexRepo(ctx context.Context, repoName, repoPath string) (IndexStats, error) {
	walkResult, err := WalkRepoDetailed(repoPath, idx.ignoreDirs)
	if err != nil {
		return IndexStats{}, fmt.Errorf("walking repo: %w", err)
	}

	stats := IndexStats{
		SkippedMinified: walkResult.SkippedMinified,
	}

	total := len(walkResult.Files)

	// Wrap all writes for this repo in a single transaction
	if err := idx.db.BeginTx(); err != nil {
		return stats, fmt.Errorf("begin transaction: %w", err)
	}
	defer idx.db.RollbackTx() // no-op if committed

	for i, filePath := range walkResult.Files {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		relPath, _ := filepath.Rel(repoPath, filePath)

		if idx.progress != nil {
			idx.progress(i+1, total, relPath)
		}

		// Fast path: check mtime+size before computing checksum
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		stateKey := filepath.Join(repoName, relPath)
		existing, _ := idx.db.GetIndexState(stateKey)
		mtimeNs := info.ModTime().UnixNano()
		fileSize := info.Size()

		if existing.Checksum != "" && existing.MtimeNs == mtimeNs && existing.FileSize == fileSize {
			stats.Unchanged++
			continue
		}

		// Stat changed — compute actual checksum to confirm
		checksum, err := fileChecksum(filePath)
		if err != nil {
			continue
		}

		if existing.Checksum == checksum {
			// Content same, just update stat cache
			idx.db.SetIndexState(stateKey, checksum, mtimeNs, fileSize)
			stats.Unchanged++
			continue
		}

		// Chunk and embed
		chunks, err := ChunkFile(repoName, filePath, DefaultChunkSize)
		if err != nil {
			continue
		}

		// Delete old embeddings for this file
		idx.db.DeleteByFile(repoName, relPath)

		// Batch embed all chunks at once
		chunkTexts := make([]string, len(chunks))
		for ci, chunk := range chunks {
			chunkTexts[ci] = chunk.Content
		}

		embeddings, err := idx.embedder.EmbedBatch(ctx, chunkTexts)
		if err != nil {
			// Context cancelled or embedder closed — skip this file
			continue
		}

		for ci, emb := range embeddings {
			if ci >= len(chunks) {
				break
			}
			embBytes := Float32ToBytes(emb)
			idx.db.StoreEmbedding(
				repoName, relPath, chunks[ci].Index,
				chunks[ci].LineStart, chunks[ci].LineEnd,
				checksum, embBytes,
			)
		}

		// Update index state with checksum and stat info
		idx.db.SetIndexState(stateKey, checksum, mtimeNs, fileSize)
		stats.Indexed++
	}

	if err := idx.db.CommitTx(); err != nil {
		return stats, fmt.Errorf("commit transaction: %w", err)
	}

	return stats, nil
}

// Search performs a semantic search across all indexed content.
func (idx *Indexer) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	queryEmb, err := idx.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	records, err := idx.db.GetAllEmbeddings()
	if err != nil {
		return nil, fmt.Errorf("loading embeddings: %w", err)
	}

	var results []SearchResult
	for _, rec := range records {
		emb := BytesToFloat32(rec.Embedding)
		if emb == nil {
			continue
		}
		score := CosineSimilarity(queryEmb, emb)
		results = append(results, SearchResult{
			Repo:      rec.Repo,
			File:      rec.File,
			LineStart: rec.LineStart,
			LineEnd:   rec.LineEnd,
			Score:     score,
		})
	}

	return RankResults(results, limit), nil
}

func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8]), nil
}
