//go:build !noembed

package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/internal/index"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/statedb"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Semantic search across all repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

var searchLimit int
var searchContent bool

func init() {
	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "max results")
	searchCmd.Flags().BoolVarP(&searchContent, "content", "c", false, "show file content for each result")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	db, err := statedb.Open(ws.Root)
	if err != nil {
		return fmt.Errorf("opening state db: %w", err)
	}
	defer db.Close()

	embedder, err := loadEmbedder(ws.Config)
	if err != nil {
		return err
	}
	defer embedder.Close()

	indexer := index.NewIndexer(db, embedder)

	results, err := indexer.Search(context.Background(), query, searchLimit)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("search", results)
		env.Summary = map[string]int{"matches": len(results)}
		return output.New("json").Format(env)
	}

	if len(results) == 0 {
		fmt.Println("No results found. Make sure to run 'mit index' first.")
		return nil
	}

	// Build repo name → abs path map for content reading
	repoPathMap := make(map[string]string)
	if searchContent {
		for _, repo := range ws.Repos {
			repoPathMap[repo.Name] = repo.AbsPath
		}
	}

	repoColor := color.New(color.FgCyan, color.Bold)
	fileColor := color.New(color.FgMagenta)
	scoreColor := color.New(color.FgGreen)
	lineNumColor := color.New(color.FgYellow)
	dimColor := color.New(color.FgHiBlack)

	for i, r := range results {
		repoColor.Fprintf(os.Stdout, "%s", r.Repo)
		fmt.Fprint(os.Stdout, ":")
		fileColor.Fprintf(os.Stdout, "%s", r.File)
		fmt.Fprintf(os.Stdout, ":%d-%d ", r.LineStart, r.LineEnd)
		scoreColor.Fprintf(os.Stdout, "(%.3f)\n", r.Score)

		if searchContent {
			if repoRoot, ok := repoPathMap[r.Repo]; ok {
				content := readFileLines(filepath.Join(repoRoot, r.File), r.LineStart, r.LineEnd)
				if content != "" {
					for li, line := range strings.Split(content, "\n") {
						lineNumColor.Fprintf(os.Stdout, "%4d", r.LineStart+li)
						dimColor.Fprint(os.Stdout, " │ ")
						fmt.Fprintln(os.Stdout, line)
					}
				}
			}
			if i < len(results)-1 {
				fmt.Println()
			}
		}
	}
	return nil
}

// readFileLines reads lines [start, end] (1-indexed) from a file.
func readFileLines(path string, start, end int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum > end {
			break
		}
		if lineNum >= start {
			lines = append(lines, scanner.Text())
		}
	}
	return strings.Join(lines, "\n")
}
