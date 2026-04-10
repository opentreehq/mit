package memory

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabemeola/mit/config"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v2"
)

// Memory represents a single memory entry stored as a markdown file with YAML frontmatter.
type Memory struct {
	ID        string   `yaml:"id"        json:"id"`
	Type      string   `yaml:"type"      json:"type"`
	Repo      string   `yaml:"repo"      json:"repo,omitempty"`
	Tags      []string `yaml:"tags"      json:"tags,omitempty"`
	CreatedBy string   `yaml:"created_by" json:"created_by,omitempty"`
	CreatedAt string   `yaml:"created_at" json:"created_at"`
	Content   string   `yaml:"-"         json:"content"`
}

// ValidTypes are the allowed memory types.
var ValidTypes = []string{"observation", "decision", "pattern", "gotcha"}

// Store manages memory files under <root>/<DataDir>/memory/.
type Store struct {
	dir string
}

// NewStore creates a Store rooted at the given workspace root.
func NewStore(workspaceRoot string) (*Store, error) {
	dir := filepath.Join(workspaceRoot, config.DataDir, "memory")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating memory directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Add persists a Memory to disk. If ID is empty, one is generated.
func (s *Store) Add(mem *Memory) error {
	if mem.Type == "" {
		mem.Type = "observation"
	}
	if !isValidType(mem.Type) {
		return fmt.Errorf("invalid memory type %q; valid types: %s", mem.Type, strings.Join(ValidTypes, ", "))
	}
	if mem.CreatedAt == "" {
		mem.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if mem.ID == "" {
		mem.ID = generateID(mem.Content)
	}

	path := filepath.Join(s.dir, mem.ID+".md")
	data, err := marshalMemory(mem)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get reads a single memory by ID.
func (s *Store) Get(id string) (*Memory, error) {
	path := filepath.Join(s.dir, id+".md")
	return parseMemoryFile(path)
}

// List returns all memories, optionally filtered by type and/or repo.
func (s *Store) List(filterType, filterRepo string) ([]Memory, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []Memory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		mem, err := parseMemoryFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue // skip unparseable files
		}
		if filterType != "" && mem.Type != filterType {
			continue
		}
		if filterRepo != "" && mem.Repo != filterRepo {
			continue
		}
		result = append(result, *mem)
	}
	return result, nil
}

// Remove deletes a memory by ID.
func (s *Store) Remove(id string) error {
	path := filepath.Join(s.dir, id+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("memory %q not found", id)
	}
	return os.Remove(path)
}

// Search performs a case-insensitive keyword search across content and tags.
func (s *Store) Search(query string) ([]Memory, error) {
	all, err := s.List("", "")
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var result []Memory
	for _, mem := range all {
		if strings.Contains(strings.ToLower(mem.Content), q) {
			result = append(result, mem)
			continue
		}
		for _, tag := range mem.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				result = append(result, mem)
				break
			}
		}
	}
	return result, nil
}

// generateID produces a short deterministic ID from content.
func generateID(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

func isValidType(t string) bool {
	for _, v := range ValidTypes {
		if v == t {
			return true
		}
	}
	return false
}

func marshalMemory(mem *Memory) ([]byte, error) {
	fm, err := yaml.Marshal(mem)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n")
	buf.WriteString(mem.Content)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func parseMemoryFile(path string) (*Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mem Memory
	content, err := frontmatter.Parse(bytes.NewReader(data), &mem)
	if err != nil {
		return nil, fmt.Errorf("parsing memory %s: %w", path, err)
	}
	mem.Content = strings.TrimSpace(string(content))

	// Derive ID from filename if not set in frontmatter.
	if mem.ID == "" {
		base := filepath.Base(path)
		mem.ID = strings.TrimSuffix(base, ".md")
	}
	return &mem, nil
}
