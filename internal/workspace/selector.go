package workspace

import "strings"

// Selector filters repos by inclusion/exclusion lists.
type Selector struct {
	Include []string
	Exclude []string
}

// NewSelector creates a selector from comma-separated repo name strings.
func NewSelector(include, exclude string) *Selector {
	return &Selector{
		Include: splitCSV(include),
		Exclude: splitCSV(exclude),
	}
}

// IsEmpty returns true if the selector has no filters.
func (s *Selector) IsEmpty() bool {
	return len(s.Include) == 0 && len(s.Exclude) == 0
}

// Matches returns true if the given repo name passes the filter.
func (s *Selector) Matches(name string) bool {
	if len(s.Exclude) > 0 {
		for _, ex := range s.Exclude {
			if ex == name {
				return false
			}
		}
	}
	if len(s.Include) > 0 {
		for _, inc := range s.Include {
			if inc == name {
				return true
			}
		}
		return false
	}
	return true
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
