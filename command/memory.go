package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opentreehq/mit/config"
	"github.com/opentreehq/mit/memory"
	"github.com/opentreehq/mit/output"
	"github.com/urfave/cli/v3"
)

// MemoryCommand returns the memory command with add, list, search, and remove subcommands.
func MemoryCommand() *cli.Command {
	return &cli.Command{
		Name:        "memory",
		Usage:       "Manage workspace memories (observations, decisions, patterns, gotchas)",
		Description: "Manage workspace memories (observations, decisions, patterns, gotchas).",
		Commands: []*cli.Command{
			{
				Name:  "add",
				Usage: "Add a new memory",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "type",
						Value: "observation",
						Usage: "memory type: observation, decision, pattern, gotcha",
						Local: true,
					},
					&cli.StringFlag{Name: "repo", Usage: "associated repo name", Local: true},
					&cli.StringFlag{Name: "tags", Usage: "comma-separated tags", Local: true},
				},
				Action: memoryAddAction,
			},
			{
				Name:  "list",
				Usage: "List all memories",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "filter by type", Local: true},
					&cli.StringFlag{Name: "repo", Usage: "filter by repo", Local: true},
				},
				Action: memoryListAction,
			},
			{
				Name:   "search",
				Usage:  "Search memories by keyword",
				Action: memorySearchAction,
			},
			{
				Name:   "remove",
				Usage:  "Remove a memory by ID",
				Action: memoryRemoveAction,
			},
		},
	}
}

func openMemoryStore() (*memory.Store, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	return memory.NewStore(root)
}

func memoryAddAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	memTags := cmd.String("tags")
	var tags []string
	if memTags != "" {
		for _, t := range strings.Split(memTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	mem := &memory.Memory{
		Type:    cmd.String("type"),
		Repo:    cmd.String("repo"),
		Tags:    tags,
		Content: cmd.Args().First(),
	}

	if err := store.Add(mem); err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("memory.add", mem)
		return output.New("json").Format(env)
	}

	fmt.Printf("Added memory %s (type=%s)\n", mem.ID, mem.Type)
	return nil
}

func memoryListAction(_ context.Context, cmd *cli.Command) error {
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	mems, err := store.List(cmd.String("type"), cmd.String("repo"))
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("memory.list", mems)
		env.Summary = map[string]int{"total": len(mems)}
		return output.New("json").Format(env)
	}

	if len(mems) == 0 {
		fmt.Println("No memories found.")
		return nil
	}

	headers := []string{"ID", "TYPE", "REPO", "TAGS", "CONTENT"}
	var rows [][]string
	for _, m := range mems {
		content := m.Content
		if len(content) > 60 {
			content = content[:57] + "..."
		}
		rows = append(rows, []string{
			m.ID,
			m.Type,
			m.Repo,
			strings.Join(m.Tags, ","),
			content,
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}

func memorySearchAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	mems, err := store.Search(cmd.Args().First())
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("memory.search", mems)
		env.Summary = map[string]int{"total": len(mems)}
		return output.New("json").Format(env)
	}

	if len(mems) == 0 {
		fmt.Println("No memories matched.")
		return nil
	}

	headers := []string{"ID", "TYPE", "REPO", "TAGS", "CONTENT"}
	var rows [][]string
	for _, m := range mems {
		content := m.Content
		if len(content) > 60 {
			content = content[:57] + "..."
		}
		rows = append(rows, []string{
			m.ID,
			m.Type,
			m.Repo,
			strings.Join(m.Tags, ","),
			content,
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}

func memoryRemoveAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	id := cmd.Args().First()
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	if err := store.Remove(id); err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("memory.remove", map[string]string{"id": id})
		return output.New("json").Format(env)
	}

	fmt.Printf("Removed memory %s\n", id)
	return nil
}
