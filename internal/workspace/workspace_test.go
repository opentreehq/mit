package workspace

import (
	"testing"
)

func TestSelector_Empty(t *testing.T) {
	sel := NewSelector("", "")
	if !sel.IsEmpty() {
		t.Error("expected empty selector")
	}
	if !sel.Matches("anything") {
		t.Error("empty selector should match everything")
	}
}

func TestSelector_Include(t *testing.T) {
	sel := NewSelector("a,b,c", "")
	tests := []struct {
		name  string
		match bool
	}{
		{"a", true},
		{"b", true},
		{"c", true},
		{"d", false},
	}
	for _, tt := range tests {
		if got := sel.Matches(tt.name); got != tt.match {
			t.Errorf("Matches(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}
}

func TestSelector_Exclude(t *testing.T) {
	sel := NewSelector("", "x,y")
	tests := []struct {
		name  string
		match bool
	}{
		{"a", true},
		{"x", false},
		{"y", false},
	}
	for _, tt := range tests {
		if got := sel.Matches(tt.name); got != tt.match {
			t.Errorf("Matches(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}
}

func TestSelector_IncludeAndExclude(t *testing.T) {
	sel := NewSelector("a,b,c", "b")
	tests := []struct {
		name  string
		match bool
	}{
		{"a", true},
		{"b", false}, // excluded takes priority
		{"c", true},
		{"d", false}, // not in include list
	}
	for _, tt := range tests {
		if got := sel.Matches(tt.name); got != tt.match {
			t.Errorf("Matches(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitCSV(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
