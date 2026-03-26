package cli

import (
	"fmt"
	"os"

	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all repos in the workspace",
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

type listItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
	URL    string `json:"url"`
	VCS    string `json:"vcs,omitempty"`
	Exists bool   `json:"exists"`
}

func runList(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return err
	}

	sel := workspace.NewSelector(flagRepos, flagExclude)
	repos := ws.FilterRepos(sel)

	var items []listItem
	for _, repo := range repos {
		vcsName := ""
		if repo.Driver != nil {
			vcsName = repo.Driver.Name()
		}
		items = append(items, listItem{
			Name:   repo.Name,
			Path:   repo.Path,
			Branch: repo.Branch,
			URL:    repo.URL,
			VCS:    vcsName,
			Exists: repo.Exists,
		})
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("list", items)
		env.Summary = map[string]int{"total": len(items)}
		return output.New("json").Format(env)
	}

	if getOutputFormat() == "plain" {
		for _, item := range items {
			fmt.Println(item.Name)
		}
		return nil
	}

	headers := []string{"NAME", "PATH", "BRANCH", "URL", "EXISTS"}
	var rows [][]string
	for _, item := range items {
		exists := "yes"
		if !item.Exists {
			exists = "no"
		}
		rows = append(rows, []string{item.Name, item.Path, item.Branch, item.URL, exists})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}
