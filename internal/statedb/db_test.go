package statedb

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := OpenPath(dbPath)
	if err != nil {
		t.Fatalf("opening temp db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := OpenPath(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
	if db.Path() != dbPath {
		t.Fatalf("path mismatch: got %q, want %q", db.Path(), dbPath)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestTaskCreateAndGet(t *testing.T) {
	db := tempDB(t)

	id, err := db.CreateTask("Fix bug", "Segfault on startup", "myrepo")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	task, err := db.GetTask(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if task.Title != "Fix bug" {
		t.Errorf("title: got %q, want %q", task.Title, "Fix bug")
	}
	if task.Description != "Segfault on startup" {
		t.Errorf("description: got %q, want %q", task.Description, "Segfault on startup")
	}
	if task.Status != "open" {
		t.Errorf("status: got %q, want %q", task.Status, "open")
	}
	if task.Repo != "myrepo" {
		t.Errorf("repo: got %q, want %q", task.Repo, "myrepo")
	}
}

func TestTaskGetNotFound(t *testing.T) {
	db := tempDB(t)
	_, err := db.GetTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestTaskList(t *testing.T) {
	db := tempDB(t)

	db.CreateTask("Task A", "", "repo1")
	db.CreateTask("Task B", "", "repo2")
	db.CreateTask("Task C", "", "repo1")

	// List all
	all, err := db.ListTasks("", "", "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list all: got %d, want 3", len(all))
	}

	// Filter by repo
	r1, err := db.ListTasks("", "", "repo1")
	if err != nil {
		t.Fatalf("list repo1: %v", err)
	}
	if len(r1) != 2 {
		t.Fatalf("list repo1: got %d, want 2", len(r1))
	}

	// Filter by status
	open, err := db.ListTasks("open", "", "")
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 3 {
		t.Fatalf("list open: got %d, want 3", len(open))
	}
}

func TestTaskClaimAtomic(t *testing.T) {
	db := tempDB(t)

	id, err := db.CreateTask("Claimable", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// First claim should succeed.
	if err := db.ClaimTask(id, "agent-1"); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	// Verify the task is claimed.
	task, _ := db.GetTask(id)
	if task.Status != "claimed" {
		t.Errorf("status after claim: got %q, want %q", task.Status, "claimed")
	}
	if task.AgentID != "agent-1" {
		t.Errorf("agent_id after claim: got %q, want %q", task.AgentID, "agent-1")
	}
	if task.ClaimedAt == nil {
		t.Error("claimed_at should be set")
	}

	// Second claim by different agent should fail (atomic guard).
	if err := db.ClaimTask(id, "agent-2"); err == nil {
		t.Fatal("expected error on second claim")
	}
}

func TestTaskUpdateStatus(t *testing.T) {
	db := tempDB(t)

	id, _ := db.CreateTask("Status test", "", "")
	db.ClaimTask(id, "agent-1")

	if err := db.UpdateTaskStatus(id, "completed"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	task, _ := db.GetTask(id)
	if task.Status != "completed" {
		t.Errorf("status: got %q, want %q", task.Status, "completed")
	}
	if task.CompletedAt == nil {
		t.Error("completed_at should be set")
	}
}

func TestTaskUpdateStatusNotFound(t *testing.T) {
	db := tempDB(t)
	if err := db.UpdateTaskStatus("nope", "done"); err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestEmbeddingStoreAndGet(t *testing.T) {
	db := tempDB(t)

	emb := []byte{0x01, 0x02, 0x03, 0x04}
	if err := db.StoreEmbedding("repo1", "main.go", 0, 1, 10, "abc123", emb); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := db.StoreEmbedding("repo1", "main.go", 1, 11, 20, "def456", emb); err != nil {
		t.Fatalf("store second: %v", err)
	}

	all, err := db.GetAllEmbeddings()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("count: got %d, want 2", len(all))
	}
	if all[0].Repo != "repo1" || all[0].File != "main.go" {
		t.Errorf("unexpected record: %+v", all[0])
	}
	if all[0].ChunkIndex != 0 || all[0].LineStart != 1 || all[0].LineEnd != 10 {
		t.Errorf("unexpected chunk data: %+v", all[0])
	}
}

func TestEmbeddingDeleteByFile(t *testing.T) {
	db := tempDB(t)

	emb := []byte{0x01}
	db.StoreEmbedding("repo1", "a.go", 0, 1, 5, "h1", emb)
	db.StoreEmbedding("repo1", "b.go", 0, 1, 5, "h2", emb)
	db.StoreEmbedding("repo2", "a.go", 0, 1, 5, "h3", emb)

	if err := db.DeleteByFile("repo1", "a.go"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	all, _ := db.GetAllEmbeddings()
	if len(all) != 2 {
		t.Fatalf("count after delete: got %d, want 2", len(all))
	}
}

func TestIndexState(t *testing.T) {
	db := tempDB(t)

	// Not indexed yet.
	state, err := db.GetIndexState("foo.go")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if state.Checksum != "" {
		t.Fatalf("expected empty checksum, got %q", state.Checksum)
	}

	// Set index state.
	if err := db.SetIndexState("foo.go", "sha256:aaa", 1000, 512); err != nil {
		t.Fatalf("set: %v", err)
	}
	state, err = db.GetIndexState("foo.go")
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if state.Checksum != "sha256:aaa" {
		t.Fatalf("checksum: got %q, want %q", state.Checksum, "sha256:aaa")
	}
	if state.MtimeNs != 1000 || state.FileSize != 512 {
		t.Fatalf("stat: got mtime=%d size=%d, want 1000/512", state.MtimeNs, state.FileSize)
	}

	// Update index state (upsert).
	if err := db.SetIndexState("foo.go", "sha256:bbb", 2000, 1024); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	state, _ = db.GetIndexState("foo.go")
	if state.Checksum != "sha256:bbb" {
		t.Fatalf("checksum after upsert: got %q, want %q", state.Checksum, "sha256:bbb")
	}
}
