package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/vcs"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

// ContextCommand returns the context subcommand.
func ContextCommand() *cli.Command {
	return &cli.Command{
		Name:        "context",
		Usage:       "Generate a workspace context document",
		Description: "Scans the workspace and generates a markdown context document with repo info, branches, and recent commits.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 5, Usage: "number of recent commits per repo", Local: true},
		},
		Action: contextAction,
	}
}

type contextRepo struct {
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	Exists      bool           `json:"exists"`
	Description string         `json:"description,omitempty"`
	Branch      string         `json:"branch,omitempty"`
	VCS         string         `json:"vcs,omitempty"`
	Commits     []contextEntry `json:"recent_commits,omitempty"`
}

type contextEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

func contextAction(ctx context.Context, cmd *cli.Command) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(cmd.String("repos"), cmd.String("exclude"))
	repos := ws.FilterRepos(sel)

	limit := cmd.Int("limit")

	var contextRepos []contextRepo
	for _, repo := range repos {
		cr := contextRepo{
			Name:   repo.Name,
			Path:   repo.Path,
			Exists: repo.Exists,
		}

		if repo.Exists {
			cr.Description = readFirstLine(filepath.Join(repo.AbsPath, "README.md"))

			driver, err := vcs.Detect(repo.AbsPath)
			if err == nil {
				cr.VCS = driver.Name()
				branch, err := driver.CurrentBranch(ctx, repo.AbsPath)
				if err == nil {
					cr.Branch = branch
				}
				commits, err := driver.Log(ctx, repo.AbsPath, limit)
				if err == nil {
					for _, c := range commits {
						cr.Commits = append(cr.Commits, contextEntry{
							Hash:    c.Hash,
							Author:  c.Author,
							Date:    c.Date,
							Message: c.Message,
						})
					}
				}
			}
		}

		contextRepos = append(contextRepos, cr)
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("context", contextRepos)
		env.Summary = map[string]any{
			"workspace": ws.Config.Workspace.Name,
			"total":     len(contextRepos),
		}
		return output.New("json").Format(env)
	}

	// Markdown output
	fmt.Printf("# Workspace: %s\n\n", ws.Config.Workspace.Name)
	if ws.Config.Workspace.Description != "" {
		fmt.Printf("%s\n\n", ws.Config.Workspace.Description)
	}

	fmt.Printf("## Repositories (%d)\n\n", len(contextRepos))

	for _, cr := range contextRepos {
		if !cr.Exists {
			fmt.Printf("### %s (not cloned)\n\n", cr.Name)
			continue
		}

		fmt.Printf("### %s\n\n", cr.Name)
		if cr.Description != "" {
			fmt.Printf("> %s\n\n", cr.Description)
		}
		fmt.Printf("- **Path:** `%s`\n", cr.Path)
		fmt.Printf("- **Branch:** `%s`\n", cr.Branch)
		fmt.Printf("- **VCS:** %s\n", cr.VCS)

		if len(cr.Commits) > 0 {
			fmt.Printf("\n**Recent commits:**\n\n")
			for _, c := range cr.Commits {
				date := c.Date
				if len(date) >= 10 {
					date = date[:10]
				}
				hash := c.Hash
				if len(hash) > 8 {
					hash = hash[:8]
				}
				fmt.Printf("- `%s` %s — %s (%s)\n", hash, date, c.Message, c.Author)
			}
		}
		fmt.Println()
	}

	return nil
}

// readFirstLine reads the first non-empty line from a file.
// Returns empty string if the file doesn't exist or can't be read.
func readFirstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip markdown heading markers for a cleaner description.
		line = strings.TrimLeft(line, "# ")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
