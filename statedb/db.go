package statedb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabemeola/mit/config"
	_ "modernc.org/sqlite"
)

// DB is a SQLite-backed state store for tasks, embeddings, and index state.
type DB struct {
	conn *sql.DB
	path string
	tx   *sql.Tx // active transaction, if any
}

// Execer abstracts *sql.DB and *sql.Tx for query execution.
func (db *DB) exec() interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
} {
	if db.tx != nil {
		return db.tx
	}
	return db.conn
}

// BeginTx starts a transaction. All subsequent DB operations will use it.
func (db *DB) BeginTx() error {
	if db.tx != nil {
		return nil // already in a transaction
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("statedb: begin transaction: %w", err)
	}
	db.tx = tx
	return nil
}

// CommitTx commits the active transaction.
func (db *DB) CommitTx() error {
	if db.tx == nil {
		return nil
	}
	err := db.tx.Commit()
	db.tx = nil
	if err != nil {
		return fmt.Errorf("statedb: commit transaction: %w", err)
	}
	return nil
}

// RollbackTx rolls back the active transaction.
func (db *DB) RollbackTx() error {
	if db.tx == nil {
		return nil
	}
	err := db.tx.Rollback()
	db.tx = nil
	return err
}

// Open opens or creates the state database at .mit/state.db under the
// workspace root found by walking up from dir.
func Open(dir string) (*DB, error) {
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("statedb: finding workspace root: %w", err)
	}
	dbDir := filepath.Join(root, ".mit")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("statedb: creating .mit directory: %w", err)
	}
	return OpenPath(filepath.Join(dbDir, "state.db"))
}

// OpenPath opens or creates the state database at an explicit file path.
func OpenPath(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("statedb: opening database: %w", err)
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("statedb: setting WAL mode: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("statedb: enabling foreign keys: %w", err)
	}
	// Performance tuning — safe with WAL mode
	conn.Exec("PRAGMA synchronous=NORMAL")
	conn.Exec("PRAGMA temp_store=MEMORY")
	conn.Exec("PRAGMA cache_size=-64000")    // 64MB page cache
	conn.Exec("PRAGMA mmap_size=268435456")  // 256MB mmap
	db := &DB{conn: conn, path: dbPath}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("statedb: running migrations: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the file path of the database.
func (db *DB) Path() string {
	return db.path
}

func (db *DB) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS tasks (
	id           TEXT PRIMARY KEY,
	title        TEXT NOT NULL,
	description  TEXT,
	status       TEXT DEFAULT 'open',
	agent_id     TEXT,
	parent_id    TEXT,
	repo         TEXT,
	created_at   DATETIME,
	claimed_at   DATETIME,
	completed_at DATETIME,
	metadata     TEXT
);

CREATE TABLE IF NOT EXISTS embeddings (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	repo         TEXT,
	file         TEXT,
	chunk_index  INT,
	line_start   INT,
	line_end     INT,
	content_hash TEXT,
	embedding    BLOB
);

CREATE TABLE IF NOT EXISTS index_state (
	file_path  TEXT PRIMARY KEY,
	checksum   TEXT,
	indexed_at DATETIME,
	mtime_ns   INTEGER DEFAULT 0,
	file_size  INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS issue_cache (
	id         TEXT NOT NULL,
	repo       TEXT NOT NULL,
	source     TEXT NOT NULL,
	title      TEXT NOT NULL,
	body       TEXT,
	status     TEXT,
	url        TEXT,
	author     TEXT,
	labels     TEXT,
	created_at DATETIME,
	fetched_at DATETIME NOT NULL,
	PRIMARY KEY (id, repo)
);
`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: add mtime_ns and file_size columns if they don't exist
	db.conn.Exec("ALTER TABLE index_state ADD COLUMN mtime_ns INTEGER DEFAULT 0")
	db.conn.Exec("ALTER TABLE index_state ADD COLUMN file_size INTEGER DEFAULT 0")

	return nil
}
