package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/gabemeola/mit/internal/config"
	"github.com/gabemeola/mit/internal/memory"
	"github.com/gabemeola/mit/internal/output"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage workspace memories (observations, decisions, patterns, gotchas)",
}

var memoryAddCmd = &cobra.Command{
	Use:   "add <content>",
	Short: "Add a new memory",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemoryAdd,
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all memories",
	RunE:  runMemoryList,
}

var memorySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories by keyword",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemorySearch,
}

var memoryRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a memory by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runMemoryRemove,
}

var (
	memType string
	memRepo string
	memTags string
)

func init() {
	memoryAddCmd.Flags().StringVar(&memType, "type", "observation", "memory type: observation, decision, pattern, gotcha")
	memoryAddCmd.Flags().StringVar(&memRepo, "repo", "", "associated repo name")
	memoryAddCmd.Flags().StringVar(&memTags, "tags", "", "comma-separated tags")

	memoryListCmd.Flags().StringVar(&memType, "type", "", "filter by type")
	memoryListCmd.Flags().StringVar(&memRepo, "repo", "", "filter by repo")

	memoryCmd.AddCommand(memoryAddCmd, memoryListCmd, memorySearchCmd, memoryRemoveCmd)
	rootCmd.AddCommand(memoryCmd)
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

func runMemoryAdd(cmd *cobra.Command, args []string) error {
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

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
		Type:    memType,
		Repo:    memRepo,
		Tags:    tags,
		Content: args[0],
	}

	if err := store.Add(mem); err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("memory.add", mem)
		return output.New("json").Format(env)
	}

	fmt.Printf("Added memory %s (type=%s)\n", mem.ID, mem.Type)
	return nil
}

func runMemoryList(cmd *cobra.Command, args []string) error {
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	mems, err := store.List(memType, memRepo)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
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

func runMemorySearch(cmd *cobra.Command, args []string) error {
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	mems, err := store.Search(args[0])
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
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

func runMemoryRemove(cmd *cobra.Command, args []string) error {
	store, err := openMemoryStore()
	if err != nil {
		return err
	}

	if err := store.Remove(args[0]); err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("memory.remove", map[string]string{"id": args[0]})
		return output.New("json").Format(env)
	}

	fmt.Printf("Removed memory %s\n", args[0])
	return nil
}
