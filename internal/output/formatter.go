package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

// Formatter is the interface for output formatting.
type Formatter interface {
	// Format writes the given data to the output.
	Format(data any) error
	// Writer returns the underlying writer.
	Writer() io.Writer
}

// Envelope is the standard JSON output envelope.
type Envelope struct {
	Version   string    `json:"version"`
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Results   any       `json:"results"`
	Summary   any       `json:"summary,omitempty"`
	Errors    []string  `json:"errors,omitempty"`
}

// NewEnvelope creates a new output envelope.
func NewEnvelope(command string, results any) *Envelope {
	return &Envelope{
		Version:   "1",
		Command:   command,
		Timestamp: time.Now().UTC(),
		Success:   true,
		Results:   results,
	}
}

// New creates a formatter for the given output mode.
func New(mode string) Formatter {
	switch mode {
	case "json":
		return &JSONFormatter{w: os.Stdout}
	case "plain":
		return &PlainFormatter{w: os.Stdout}
	default:
		return &TableFormatter{w: os.Stdout}
	}
}

// JSONFormatter outputs JSON.
type JSONFormatter struct {
	w io.Writer
}

func (f *JSONFormatter) Format(data any) error {
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func (f *JSONFormatter) Writer() io.Writer { return f.w }

// PlainFormatter outputs plain text, one item per line.
type PlainFormatter struct {
	w io.Writer
}

func (f *PlainFormatter) Format(data any) error {
	switch v := data.(type) {
	case []string:
		for _, s := range v {
			fmt.Fprintln(f.w, s)
		}
	case string:
		fmt.Fprintln(f.w, v)
	default:
		// Fall back to JSON for complex types
		enc := json.NewEncoder(f.w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	return nil
}

func (f *PlainFormatter) Writer() io.Writer { return f.w }

// TableFormatter outputs human-readable tables.
type TableFormatter struct {
	w io.Writer
}

func (f *TableFormatter) Format(data any) error {
	switch v := data.(type) {
	case *Envelope:
		return f.formatEnvelope(v)
	default:
		// Fall back to JSON for unknown types
		enc := json.NewEncoder(f.w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
}

func (f *TableFormatter) Writer() io.Writer { return f.w }

func (f *TableFormatter) formatEnvelope(env *Envelope) error {
	if env.Errors != nil && len(env.Errors) > 0 {
		errColor := color.New(color.FgRed)
		for _, e := range env.Errors {
			errColor.Fprintf(f.w, "error: %s\n", e)
		}
	}
	// The caller is expected to format results before passing to the table formatter
	// This is a fallback
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(env.Results)
}

// PrintTable writes a styled table to the given writer using lipgloss.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, col := range row {
			if i < len(widths) && len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}

	// Header
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, headerStyle.Render(fmt.Sprintf("%-*s", widths[i], h)))
	}
	fmt.Fprintln(w)

	// Separator
	var sep []string
	for _, width := range widths {
		sep = append(sep, strings.Repeat("─", width))
	}
	fmt.Fprintln(w, sepStyle.Render(strings.Join(sep, "  ")))

	// Rows
	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			if i < len(widths) {
				fmt.Fprintf(w, "%-*s", widths[i], col)
			}
		}
		fmt.Fprintln(w)
	}
}
