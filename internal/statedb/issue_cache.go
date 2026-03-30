package statedb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CachedIssue represents a cached remote issue.
type CachedIssue struct {
	ID        string    `json:"id"`
	Repo      string    `json:"repo"`
	Source    string    `json:"source"` // "github" or "gitlab"
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Status    string    `json:"status"`
	URL       string    `json:"url,omitempty"`
	Author    string    `json:"author,omitempty"`
	Labels    string    `json:"labels,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	FetchedAt time.Time `json:"fetched_at"`
}

// CacheIssues replaces the cached issues for a given repo with fresh data.
func (db *DB) CacheIssues(repo string, issues []CachedIssue) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("statedb: cache issues begin: %w", err)
	}
	defer tx.Rollback()

	// Delete old cache for this repo
	if _, err := tx.Exec("DELETE FROM issue_cache WHERE repo = ?", repo); err != nil {
		return fmt.Errorf("statedb: cache issues delete: %w", err)
	}

	// Insert new issues — store timestamps as Unix seconds for reliable round-tripping.
	for _, issue := range issues {
		_, err := tx.Exec(
			`INSERT INTO issue_cache (id, repo, source, title, body, status, url, author, labels, created_at, fetched_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			issue.ID, issue.Repo, issue.Source, issue.Title, issue.Body,
			issue.Status, issue.URL, issue.Author, issue.Labels,
			issue.CreatedAt.Unix(), issue.FetchedAt.Unix(),
		)
		if err != nil {
			return fmt.Errorf("statedb: cache issues insert: %w", err)
		}
	}

	return tx.Commit()
}

// GetCachedIssues returns cached issues, optionally filtered by repo.
func (db *DB) GetCachedIssues(repo string) ([]CachedIssue, error) {
	query := `SELECT id, repo, source, title, body, status, url, author, labels, created_at, fetched_at
	          FROM issue_cache`
	var args []any
	if repo != "" {
		query += " WHERE repo = ?"
		args = append(args, repo)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("statedb: get cached issues: %w", err)
	}
	defer rows.Close()

	var issues []CachedIssue
	for rows.Next() {
		var ci CachedIssue
		var body, url, author, labels sql.NullString
		var createdUnix, fetchedUnix int64
		if err := rows.Scan(&ci.ID, &ci.Repo, &ci.Source, &ci.Title, &body,
			&ci.Status, &url, &author, &labels, &createdUnix, &fetchedUnix); err != nil {
			return nil, fmt.Errorf("statedb: scanning cached issue: %w", err)
		}
		ci.Body = body.String
		ci.URL = url.String
		ci.Author = author.String
		ci.Labels = labels.String
		ci.CreatedAt = time.Unix(createdUnix, 0).UTC()
		ci.FetchedAt = time.Unix(fetchedUnix, 0).UTC()
		issues = append(issues, ci)
	}
	return issues, rows.Err()
}

// GetCacheAge returns the age of the oldest cached entry for the given repos.
// Returns 0 if cache is empty. If repos is empty, checks all cached repos.
func (db *DB) GetCacheAge(repos []string) time.Duration {
	query := "SELECT MIN(fetched_at) FROM issue_cache"
	var args []any
	if len(repos) > 0 {
		placeholders := make([]string, len(repos))
		for i, r := range repos {
			placeholders[i] = "?"
			args = append(args, r)
		}
		query += " WHERE repo IN (" + strings.Join(placeholders, ",") + ")"
	}

	var fetchedUnix sql.NullInt64
	if err := db.conn.QueryRow(query, args...).Scan(&fetchedUnix); err != nil || !fetchedUnix.Valid {
		return 0
	}
	return time.Since(time.Unix(fetchedUnix.Int64, 0))
}

// ClearIssueCache removes all cached issues.
func (db *DB) ClearIssueCache() error {
	_, err := db.conn.Exec("DELETE FROM issue_cache")
	return err
}
