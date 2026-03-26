package cli

import (
	"fmt"
	"os"

	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/statedb"
	"github.com/spf13/cobra"
)

var (
	taskFlagDescription string
	taskFlagRepo        string
	taskFlagStatus      string
	taskFlagAgent       string
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks for agents and humans",
}

var taskCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskCreate,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	Args:  cobra.NoArgs,
	RunE:  runTaskList,
}

var taskClaimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a task for an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskClaim,
}

var taskUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a task's status",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskUpdate,
}

var taskShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskShow,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskClaimCmd)
	taskCmd.AddCommand(taskUpdateCmd)
	taskCmd.AddCommand(taskShowCmd)

	taskCreateCmd.Flags().StringVar(&taskFlagDescription, "description", "", "task description")
	taskCreateCmd.Flags().StringVar(&taskFlagRepo, "repo", "", "associated repo")

	taskListCmd.Flags().StringVar(&taskFlagStatus, "status", "", "filter by status")
	taskListCmd.Flags().StringVar(&taskFlagAgent, "agent", "", "filter by agent id")
	taskListCmd.Flags().StringVar(&taskFlagRepo, "repo", "", "filter by repo")

	taskClaimCmd.Flags().StringVar(&taskFlagAgent, "agent", "", "agent id to claim as")
	taskClaimCmd.MarkFlagRequired("agent")

	taskUpdateCmd.Flags().StringVar(&taskFlagStatus, "status", "", "new status")
	taskUpdateCmd.MarkFlagRequired("status")
}

func openStateDB() (*statedb.DB, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return statedb.Open(dir)
}

func runTaskCreate(cmd *cobra.Command, args []string) error {
	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	title := args[0]
	id, err := db.CreateTask(title, taskFlagDescription, taskFlagRepo)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		task, err := db.GetTask(id)
		if err != nil {
			return err
		}
		env := output.NewEnvelope("task create", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("Created task %s\n", id)
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	tasks, err := db.ListTasks(taskFlagStatus, taskFlagAgent, taskFlagRepo)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task list", tasks)
		env.Summary = map[string]int{"total": len(tasks)}
		return output.New("json").Format(env)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	headers := []string{"ID", "TITLE", "STATUS", "AGENT", "REPO"}
	var rows [][]string
	for _, t := range tasks {
		rows = append(rows, []string{
			shortID(t.ID),
			t.Title,
			t.Status,
			t.AgentID,
			t.Repo,
		})
	}
	output.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runTaskClaim(cmd *cobra.Command, args []string) error {
	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	id := args[0]
	if err := db.ClaimTask(id, taskFlagAgent); err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		task, err := db.GetTask(id)
		if err != nil {
			return err
		}
		env := output.NewEnvelope("task claim", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("Task %s claimed by %s\n", shortID(id), taskFlagAgent)
	return nil
}

func runTaskUpdate(cmd *cobra.Command, args []string) error {
	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	id := args[0]
	if err := db.UpdateTaskStatus(id, taskFlagStatus); err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		task, err := db.GetTask(id)
		if err != nil {
			return err
		}
		env := output.NewEnvelope("task update", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("Task %s status updated to %s\n", shortID(id), taskFlagStatus)
	return nil
}

func runTaskShow(cmd *cobra.Command, args []string) error {
	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	id := args[0]
	task, err := db.GetTask(id)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task show", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("ID:          %s\n", task.ID)
	fmt.Printf("Title:       %s\n", task.Title)
	fmt.Printf("Status:      %s\n", task.Status)
	if task.Description != "" {
		fmt.Printf("Description: %s\n", task.Description)
	}
	if task.AgentID != "" {
		fmt.Printf("Agent:       %s\n", task.AgentID)
	}
	if task.ParentID != "" {
		fmt.Printf("Parent:      %s\n", task.ParentID)
	}
	if task.Repo != "" {
		fmt.Printf("Repo:        %s\n", task.Repo)
	}
	fmt.Printf("Created:     %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if task.ClaimedAt != nil {
		fmt.Printf("Claimed:     %s\n", task.ClaimedAt.Format("2006-01-02 15:04:05"))
	}
	if task.CompletedAt != nil {
		fmt.Printf("Completed:   %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if task.Metadata != "" {
		fmt.Printf("Metadata:    %s\n", task.Metadata)
	}
	return nil
}

// shortID returns the first 8 characters of a UUID for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
