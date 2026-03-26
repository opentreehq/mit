package statedb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Task represents a row in the tasks table.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	AgentID     string     `json:"agent_id,omitempty"`
	ParentID    string     `json:"parent_id,omitempty"`
	Repo        string     `json:"repo,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ClaimedAt   *time.Time `json:"claimed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Metadata    string     `json:"metadata,omitempty"`
}

// CreateTask inserts a new task with status "open" and returns its UUID.
func (db *DB) CreateTask(title, description, repo string) (string, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := db.conn.Exec(
		`INSERT INTO tasks (id, title, description, status, repo, created_at)
		 VALUES (?, ?, ?, 'open', ?, ?)`,
		id, title, description, repo, now,
	)
	if err != nil {
		return "", fmt.Errorf("statedb: creating task: %w", err)
	}
	return id, nil
}

// ListTasks returns tasks matching the optional filters. Pass empty strings to
// skip a filter.
func (db *DB) ListTasks(status, agentID, repo string) ([]Task, error) {
	query := "SELECT id, title, description, status, agent_id, parent_id, repo, created_at, claimed_at, completed_at, metadata FROM tasks"
	var clauses []string
	var args []any

	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	if agentID != "" {
		clauses = append(clauses, "agent_id = ?")
		args = append(args, agentID)
	}
	if repo != "" {
		clauses = append(clauses, "repo = ?")
		args = append(args, repo)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("statedb: listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var desc, agentid, parentid, repo, meta sql.NullString
		var claimed, completed sql.NullTime
		if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &agentid, &parentid, &repo, &t.CreatedAt, &claimed, &completed, &meta); err != nil {
			return nil, fmt.Errorf("statedb: scanning task: %w", err)
		}
		t.Description = desc.String
		t.AgentID = agentid.String
		t.ParentID = parentid.String
		t.Repo = repo.String
		t.Metadata = meta.String
		if claimed.Valid {
			t.ClaimedAt = &claimed.Time
		}
		if completed.Valid {
			t.CompletedAt = &completed.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTask returns a single task by ID, or an error if not found.
func (db *DB) GetTask(id string) (*Task, error) {
	row := db.conn.QueryRow(
		`SELECT id, title, description, status, agent_id, parent_id, repo, created_at, claimed_at, completed_at, metadata
		 FROM tasks WHERE id = ?`, id,
	)
	var t Task
	var desc, agentid, parentid, repo, meta sql.NullString
	var claimed, completed sql.NullTime
	if err := row.Scan(&t.ID, &t.Title, &desc, &t.Status, &agentid, &parentid, &repo, &t.CreatedAt, &claimed, &completed, &meta); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("statedb: task %q not found", id)
		}
		return nil, fmt.Errorf("statedb: getting task: %w", err)
	}
	t.Description = desc.String
	t.AgentID = agentid.String
	t.ParentID = parentid.String
	t.Repo = repo.String
	t.Metadata = meta.String
	if claimed.Valid {
		t.ClaimedAt = &claimed.Time
	}
	if completed.Valid {
		t.CompletedAt = &completed.Time
	}
	return &t, nil
}

// ClaimTask atomically assigns a task to an agent. It only succeeds when the
// task has status "open" and agent_id IS NULL. Returns an error if the task
// was already claimed or does not exist.
func (db *DB) ClaimTask(id, agentID string) error {
	now := time.Now().UTC()
	res, err := db.conn.Exec(
		`UPDATE tasks SET agent_id = ?, status = 'claimed', claimed_at = ?
		 WHERE id = ? AND status = 'open' AND agent_id IS NULL`,
		agentID, now, id,
	)
	if err != nil {
		return fmt.Errorf("statedb: claiming task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("statedb: claiming task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("statedb: task %q is not available for claiming", id)
	}
	return nil
}

// UpdateTaskStatus changes the status of a task. If the new status is
// "completed", the completed_at timestamp is set.
func (db *DB) UpdateTaskStatus(id, status string) error {
	var res sql.Result
	var err error
	if status == "completed" {
		now := time.Now().UTC()
		res, err = db.conn.Exec(
			`UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?`,
			status, now, id,
		)
	} else {
		res, err = db.conn.Exec(
			`UPDATE tasks SET status = ? WHERE id = ?`,
			status, id,
		)
	}
	if err != nil {
		return fmt.Errorf("statedb: updating task status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("statedb: updating task status: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("statedb: task %q not found", id)
	}
	return nil
}
