//go:build !noembed

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gabemeola/mit/internal/index"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/statedb"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Build/update semantic embeddings index",
	RunE:  runIndex,
}

var indexRebuild bool
var indexStatus bool

func init() {
	indexCmd.Flags().BoolVar(&indexRebuild, "rebuild", false, "force full rebuild")
	indexCmd.Flags().BoolVar(&indexStatus, "status", false, "show index health")
	rootCmd.AddCommand(indexCmd)
}

func runIndex(cmd *cobra.Command, args []string) error {
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

	if indexStatus {
		return showIndexStatus(db, ws)
	}

	embedder, err := loadEmbedder(ws.Config)
	if err != nil {
		return err
	}
	defer embedder.Close()

	indexer := index.NewIndexer(db, embedder)
	indexer.SetIgnoreDirs(ws.Config.IndexIgnoreSet())

	sel := workspace.NewSelector(flagRepos, flagExclude)
	repos := ws.FilterRepos(sel)

	totalIndexed := 0
	totalUnchanged := 0
	totalMinified := 0
	overallStart := time.Now()

	for ri, repo := range repos {
		if !repo.Exists {
			continue
		}

		if flagDryRun {
			files, _ := index.WalkRepo(repo.AbsPath, ws.Config.IndexIgnoreSet())
			dryRunMsg("would index %d files in %s", len(files), repo.Name)
			continue
		}

		if !flagQuiet {
			fmt.Printf("\n\033[1m[%d/%d] %s\033[0m\n", ri+1, len(repos), repo.Name)
		}

		if !flagQuiet {
			indexer.SetProgress(func(current, total int, file string) {
				fmt.Fprintf(os.Stderr, "\r  \033[33m%d/%d\033[0m %s\033[K", current, total, file)
			})
		}

		repoStart := time.Now()
		stats, err := indexer.IndexRepo(context.Background(), repo.Name, repo.AbsPath)

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "\r\033[K") // clear progress line
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "  \033[31merror: %v\033[0m\n", err)
			continue
		}

		if !flagQuiet {
			elapsed := time.Since(repoStart)
			fmt.Printf("  %d indexed, %d unchanged (%s)\n", stats.Indexed, stats.Unchanged, formatElapsed(elapsed))

			if len(stats.SkippedMinified) > 0 {
				fmt.Printf("  \033[33mskipped %d minified file(s):\033[0m", len(stats.SkippedMinified))
				for _, f := range stats.SkippedMinified {
					fmt.Printf(" %s", f)
				}
				fmt.Println()
			}
		}
		totalIndexed += stats.Indexed
		totalUnchanged += stats.Unchanged
		totalMinified += len(stats.SkippedMinified)
	}

	if !flagDryRun {
		elapsed := time.Since(overallStart)
		fmt.Printf("\nDone: %d files indexed, %d unchanged", totalIndexed, totalUnchanged)
		if totalMinified > 0 {
			fmt.Printf(", %d minified skipped", totalMinified)
		}
		fmt.Printf(" (%s)\n", formatElapsed(elapsed))
	}
	return nil
}

func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	return fmt.Sprintf("%dm%ds", m, s)
}

func showIndexStatus(db *statedb.DB, ws *workspace.Workspace) error {
	records, err := db.GetAllEmbeddings()
	if err != nil {
		return err
	}

	type repoStats struct {
		Files  map[string]bool
		Chunks int
	}
	stats := make(map[string]*repoStats)
	for _, r := range records {
		if stats[r.Repo] == nil {
			stats[r.Repo] = &repoStats{Files: make(map[string]bool)}
		}
		stats[r.Repo].Files[r.File] = true
		stats[r.Repo].Chunks++
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("index status", stats)
		return output.New("json").Format(env)
	}

	if len(stats) == 0 {
		fmt.Println("Index is empty. Run 'mit index' to build it.")
		return nil
	}

	headers := []string{"REPO", "FILES", "CHUNKS"}
	var rows [][]string
	for repo, s := range stats {
		rows = append(rows, []string{
			repo,
			fmt.Sprintf("%d", len(s.Files)),
			fmt.Sprintf("%d", s.Chunks),
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}
