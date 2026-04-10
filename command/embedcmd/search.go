package embedcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/gabemeola/mit/index"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/statedb"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Semantic search across all repos",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Value: 10, Usage: "max results", Local: true},
			&cli.BoolFlag{Name: "content", Aliases: []string{"c"}, Usage: "show file content for each result", Local: true},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected 1 argument(s), received %d", cmd.Args().Len())
			}
			query := cmd.Args().First()

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

			quiet := cmd.Bool("quiet")
			embedder, err := loadEmbedder(ws.Config, quiet)
			if err != nil {
				return err
			}
			defer embedder.Close()

			indexer := index.NewIndexer(db, embedder)

			limit := int(cmd.Int("limit"))
			results, err := indexer.Search(ctx, query, limit)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			outputFormat := cmd.String("output")
			if outputFormat == "json" {
				env := output.NewEnvelope("search", results)
				env.Summary = map[string]int{"matches": len(results)}
				return output.New("json").Format(env)
			}

			if len(results) == 0 {
				fmt.Println("No results found. Make sure to run 'mit index' first.")
				return nil
			}

			showContent := cmd.Bool("content")
			repoPathMap := make(map[string]string)
			if showContent {
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

				if showContent {
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
		},
	}
}

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
