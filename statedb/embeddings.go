package statedb

import (
	"database/sql"
	"fmt"
	"time"
)

// EmbeddingRecord represents a row in the embeddings table.
type EmbeddingRecord struct {
	ID          int64  `json:"id"`
	Repo        string `json:"repo"`
	File        string `json:"file"`
	ChunkIndex  int    `json:"chunk_index"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	ContentHash string `json:"content_hash"`
	Embedding   []byte `json:"embedding"`
}

// StoreEmbedding inserts a new embedding record.
func (db *DB) StoreEmbedding(repo, file string, chunkIndex, lineStart, lineEnd int, contentHash string, embedding []byte) error {
	_, err := db.exec().Exec(
		`INSERT INTO embeddings (repo, file, chunk_index, line_start, line_end, content_hash, embedding)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		repo, file, chunkIndex, lineStart, lineEnd, contentHash, embedding,
	)
	if err != nil {
		return fmt.Errorf("statedb: storing embedding: %w", err)
	}
	return nil
}

// GetAllEmbeddings returns every embedding record.
func (db *DB) GetAllEmbeddings() ([]EmbeddingRecord, error) {
	rows, err := db.exec().Query(
		`SELECT id, repo, file, chunk_index, line_start, line_end, content_hash, embedding FROM embeddings`,
	)
	if err != nil {
		return nil, fmt.Errorf("statedb: querying embeddings: %w", err)
	}
	defer rows.Close()

	var records []EmbeddingRecord
	for rows.Next() {
		var r EmbeddingRecord
		if err := rows.Scan(&r.ID, &r.Repo, &r.File, &r.ChunkIndex, &r.LineStart, &r.LineEnd, &r.ContentHash, &r.Embedding); err != nil {
			return nil, fmt.Errorf("statedb: scanning embedding: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// DeleteByFile removes all embedding records for a given repo and file.
func (db *DB) DeleteByFile(repo, file string) error {
	_, err := db.exec().Exec(
		`DELETE FROM embeddings WHERE repo = ? AND file = ?`,
		repo, file,
	)
	if err != nil {
		return fmt.Errorf("statedb: deleting embeddings by file: %w", err)
	}
	return nil
}

// IndexState holds the stored state for a file in the index.
type IndexState struct {
	Checksum string
	MtimeNs  int64
	FileSize int64
}

// GetIndexState returns the stored state for a file path. If the file has
// not been indexed, it returns a zero IndexState and a nil error.
func (db *DB) GetIndexState(filePath string) (IndexState, error) {
	var s IndexState
	err := db.exec().QueryRow(
		`SELECT checksum, mtime_ns, file_size FROM index_state WHERE file_path = ?`, filePath,
	).Scan(&s.Checksum, &s.MtimeNs, &s.FileSize)
	if err == sql.ErrNoRows {
		return IndexState{}, nil
	}
	if err != nil {
		return IndexState{}, fmt.Errorf("statedb: getting index state: %w", err)
	}
	return s, nil
}

// SetIndexState upserts the state for a file path.
func (db *DB) SetIndexState(filePath, checksum string, mtimeNs, fileSize int64) error {
	now := time.Now().UTC()
	_, err := db.exec().Exec(
		`INSERT INTO index_state (file_path, checksum, indexed_at, mtime_ns, file_size) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(file_path) DO UPDATE SET checksum = excluded.checksum, indexed_at = excluded.indexed_at, mtime_ns = excluded.mtime_ns, file_size = excluded.file_size`,
		filePath, checksum, now, mtimeNs, fileSize,
	)
	if err != nil {
		return fmt.Errorf("statedb: setting index state: %w", err)
	}
	return nil
}
