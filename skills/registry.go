package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opentreehq/mit/config"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v2"
)

// Skill represents a reusable skill stored as a markdown file with YAML frontmatter.
type Skill struct {
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	Triggers    []string `yaml:"triggers"    json:"triggers,omitempty"`
	Repos       []string `yaml:"repos"       json:"repos,omitempty"`
	Content     string   `yaml:"-"           json:"content"`
}

// Registry manages skill files under <root>/<DataDir>/skills/.
type Registry struct {
	dir string
}

// NewRegistry creates a Registry rooted at the given workspace root.
func NewRegistry(workspaceRoot string) (*Registry, error) {
	dir := filepath.Join(workspaceRoot, config.DataDir, "skills")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating skills directory: %w", err)
	}
	return &Registry{dir: dir}, nil
}

// Create writes a new skill to disk.
func (r *Registry) Create(skill *Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	safeName := sanitizeName(skill.Name)
	path := filepath.Join(r.dir, safeName+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("skill %q already exists", skill.Name)
	}

	data, err := marshalSkill(skill)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get reads a single skill by name.
func (r *Registry) Get(name string) (*Skill, error) {
	safeName := sanitizeName(name)
	path := filepath.Join(r.dir, safeName+".md")
	return parseSkillFile(path)
}

// List returns all skills in the registry.
func (r *Registry) List() ([]Skill, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []Skill
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		skill, err := parseSkillFile(filepath.Join(r.dir, entry.Name()))
		if err != nil {
			continue // skip unparseable files
		}
		result = append(result, *skill)
	}
	return result, nil
}

// Search performs a case-insensitive keyword search across name, description, and triggers.
func (r *Registry) Search(query string) ([]Skill, error) {
	all, err := r.List()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var result []Skill
	for _, skill := range all {
		if strings.Contains(strings.ToLower(skill.Name), q) ||
			strings.Contains(strings.ToLower(skill.Description), q) {
			result = append(result, skill)
			continue
		}
		for _, trigger := range skill.Triggers {
			if strings.Contains(strings.ToLower(trigger), q) {
				result = append(result, skill)
				break
			}
		}
	}
	return result, nil
}

// sanitizeName converts a skill name to a safe filename component.
func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	// Remove characters that are unsafe for filenames.
	var safe strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			safe.WriteRune(r)
		}
	}
	return safe.String()
}

func marshalSkill(skill *Skill) ([]byte, error) {
	fm, err := yaml.Marshal(skill)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n")
	buf.WriteString(skill.Content)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func parseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var skill Skill
	content, err := frontmatter.Parse(bytes.NewReader(data), &skill)
	if err != nil {
		return nil, fmt.Errorf("parsing skill %s: %w", path, err)
	}
	skill.Content = strings.TrimSpace(string(content))

	// Derive name from filename if not set in frontmatter.
	if skill.Name == "" {
		base := filepath.Base(path)
		skill.Name = strings.TrimSuffix(base, ".md")
	}
	return &skill, nil
}
