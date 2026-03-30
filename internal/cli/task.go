package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gabemeola/mit/internal/forge"
	"github.com/gabemeola/mit/internal/output"
	"github.com/gabemeola/mit/internal/statedb"
	"github.com/gabemeola/mit/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	taskFlagDescription string
	taskFlagRepo        string
	taskFlagStatus      string
	taskFlagAgent       string
	taskFlagSource      string
	taskFlagRemote      bool
	taskFlagRefresh     bool
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
	taskCreateCmd.Flags().BoolVar(&taskFlagRemote, "remote", false, "create issue on forge instead of locally")

	taskListCmd.Flags().StringVar(&taskFlagStatus, "status", "", "filter by status")
	taskListCmd.Flags().StringVar(&taskFlagAgent, "agent", "", "filter by agent id")
	taskListCmd.Flags().StringVar(&taskFlagRepo, "repo", "", "filter by repo")
	taskListCmd.Flags().StringVar(&taskFlagSource, "source", "all", "task source: local, remote, all")
	taskListCmd.Flags().BoolVar(&taskFlagRefresh, "refresh", false, "force refresh of cached remote issues")

	taskClaimCmd.Flags().StringVar(&taskFlagAgent, "agent", "", "agent id to claim as")
	taskClaimCmd.Flags().StringVar(&taskFlagRepo, "repo", "", "repo for remote issues")
	taskClaimCmd.MarkFlagRequired("agent")

	taskUpdateCmd.Flags().StringVar(&taskFlagStatus, "status", "", "new status")
	taskUpdateCmd.MarkFlagRequired("status")

	taskShowCmd.Flags().StringVar(&taskFlagRepo, "repo", "", "repo for remote issues")
}

func openStateDB() (*statedb.DB, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return statedb.Open(dir)
}

func runTaskCreate(cmd *cobra.Command, args []string) error {
	title := args[0]

	if taskFlagRemote {
		return runTaskCreateRemote(title)
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

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

func runTaskCreateRemote(title string) error {
	if taskFlagRepo == "" {
		return fmt.Errorf("--repo is required when creating remote issues")
	}

	ws, f, owner, repo, err := resolveRepoForge(taskFlagRepo)
	if err != nil {
		return err
	}
	_ = ws

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	issue, err := f.CreateIssue(context.Background(), owner, repo, title, taskFlagDescription)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task create", issue)
		return output.New("json").Format(env)
	}

	fmt.Printf("Created issue: %s\n", issue.URL)
	return nil
}

type taskRow struct {
	statedb.Task
	Source string `json:"source"`
}

func runTaskList(cmd *cobra.Command, args []string) error {
	var rows []taskRow

	// Local tasks
	if taskFlagSource == "local" || taskFlagSource == "all" {
		db, err := openStateDB()
		if err != nil {
			if taskFlagSource == "local" {
				return err
			}
			fmt.Fprintf(os.Stderr, "warning: could not open local task db: %v\n", err)
		} else {
			defer db.Close()
			tasks, err := db.ListTasks(taskFlagStatus, taskFlagAgent, taskFlagRepo)
			if err != nil {
				return err
			}
			for _, t := range tasks {
				rows = append(rows, taskRow{Task: t, Source: "local"})
			}
		}
	}

	// Remote tasks — stale-while-revalidate.
	// Always show cached data instantly, refresh in background for next run.
	if taskFlagSource == "remote" || taskFlagSource == "all" {
		remote, err := getRemoteTasks()
		if err != nil {
			if taskFlagSource == "remote" {
				return err
			}
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		rows = append(rows, remote...)
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task list", rows)
		env.Summary = map[string]int{"total": len(rows)}
		return output.New("json").Format(env)
	}

	if len(rows) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	headers := []string{"ID", "TITLE", "STATUS", "AGENT", "REPO", "SOURCE"}
	var tableRows [][]string
	for _, r := range rows {
		tableRows = append(tableRows, []string{
			shortID(r.ID),
			r.Title,
			r.Status,
			r.AgentID,
			r.Repo,
			r.Source,
		})
	}
	output.PrintTable(os.Stdout, headers, tableRows)
	return nil
}

// getRemoteTasks returns remote issues using a stale-while-revalidate strategy:
//   - If cache exists: return cached data instantly. If stale, refresh in background
//     so the next run has fresh data.
//   - If no cache or --refresh: fetch synchronously and cache.
func getRemoteTasks() ([]taskRow, error) {
	db, err := openStateDB()
	if err != nil {
		// Can't cache — fall back to direct fetch.
		return fetchRemoteTasksDirect()
	}

	// Try serving from cache.
	if !taskFlagRefresh {
		cached, err := db.GetCachedIssues(taskFlagRepo)
		if err == nil && len(cached) > 0 {
			// Serve cached data immediately.
			rows := cachedToRows(cached)

			// If stale, kick off background refresh for next run.
			cacheAge := db.GetCacheAge(nil)
			if cacheAge <= 0 || cacheAge > 5*time.Minute {
				go func() {
					bgDB, err := openStateDB()
					if err != nil {
						return
					}
					defer bgDB.Close()
					fetchAndCacheRemoteTasks(bgDB)
				}()
			}

			db.Close()
			return rows, nil
		}
	}

	// No cache — fetch synchronously (first run or --refresh).
	rows, err := fetchAndCacheRemoteTasks(db)
	db.Close()
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func cachedToRows(cached []statedb.CachedIssue) []taskRow {
	var rows []taskRow
	for _, ci := range cached {
		rows = append(rows, taskRow{
			Task: statedb.Task{
				ID:        ci.ID,
				Title:     ci.Title,
				Description: ci.Body,
				Status:    ci.Status,
				Repo:      ci.Repo,
				CreatedAt: ci.CreatedAt,
				Metadata:  ci.URL,
			},
			Source: ci.Source,
		})
	}
	return rows
}

type resolvedForgeRepo struct {
	repo     workspace.RepoInfo
	forge    forge.Forge
	owner    string
	repoName string
}

// resolveForgeRepos resolves forge connections for all repos, deduplicating warnings.
func resolveForgeRepos() ([]resolvedForgeRepo, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}

	sel := workspace.NewSelector(taskFlagRepo, "")
	repos := ws.FilterRepos(sel)

	warned := map[string]bool{}
	warnOnce := func(msg string) {
		if !warned[msg] {
			fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
			warned[msg] = true
		}
	}

	var resolved []resolvedForgeRepo
	for _, repo := range repos {
		if repo.Forge == "" {
			if taskFlagRepo != "" {
				return nil, fmt.Errorf("no forge configured for repo %q; set 'forge: github' or 'forge: gitlab' in mit.yaml", repo.Name)
			}
			continue
		}

		f, owner, repoName, err := forge.ResolveForge(repo.Name, repo.URL, repo.Forge)
		if err != nil {
			warnOnce(err.Error())
			continue
		}

		if err := f.CheckAvailable(); err != nil {
			warnOnce(err.Error())
			continue
		}

		if err := f.CheckAuthenticated(); err != nil {
			warnOnce(err.Error())
			continue
		}

		resolved = append(resolved, resolvedForgeRepo{repo, f, owner, repoName})
	}

	return resolved, nil
}

// repoResult holds the fetch result for a single repo.
type repoResult struct {
	repoName string
	rows     []taskRow
	cached   []statedb.CachedIssue
	err      error
}

// fetchAndCacheRemoteTasks fetches issues from forges in parallel, caches them, and returns rows.
func fetchAndCacheRemoteTasks(db *statedb.DB) ([]taskRow, error) {
	resolved, err := resolveForgeRepos()
	if err != nil {
		return nil, err
	}

	if len(resolved) == 0 {
		return nil, nil
	}

	showProgress := getOutputFormat() != "json"
	now := time.Now().UTC()

	// Fetch all repos in parallel.
	results := make([]repoResult, len(resolved))
	var done sync.WaitGroup
	var doneCount int64
	var mu sync.Mutex

	for i, r := range resolved {
		done.Add(1)
		go func(idx int, r resolvedForgeRepo) {
			defer done.Done()
			res := repoResult{repoName: r.repo.Name}

			issues, err := r.forge.ListIssues(context.Background(), r.owner, r.repoName, "open")
			if err != nil {
				res.err = err
				results[idx] = res

				mu.Lock()
				doneCount++
				mu.Unlock()
				return
			}

			ft := r.forge.Type()
			for _, issue := range issues {
				res.rows = append(res.rows, taskRow{
					Task:   issueToTask(issue, r.repo.Name, ft),
					Source: string(ft),
				})
				res.cached = append(res.cached, statedb.CachedIssue{
					ID:        forge.FormatRemoteID(ft, issue.Number),
					Repo:      r.repo.Name,
					Source:    string(ft),
					Title:     issue.Title,
					Body:      issue.Body,
					Status:    issue.State,
					URL:       issue.URL,
					Author:    issue.Author,
					Labels:    strings.Join(issue.Labels, ","),
					CreatedAt: issue.CreatedAt,
					FetchedAt: now,
				})
			}
			results[idx] = res

			mu.Lock()
			doneCount++
			if showProgress {
				fmt.Fprintf(os.Stderr, "\r\033[Kfetching issues: %d/%d repos", doneCount, len(resolved))
			}
			mu.Unlock()
		}(i, r)
	}

	if showProgress {
		fmt.Fprintf(os.Stderr, "fetching issues: 0/%d repos", len(resolved))
	}
	done.Wait()
	if showProgress {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}

	// Collect results and cache.
	warned := map[string]bool{}
	var rows []taskRow
	for _, res := range results {
		if res.err != nil {
			msg := fmt.Sprintf("listing issues for %s: %v", res.repoName, res.err)
			if !warned[msg] {
				fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
				warned[msg] = true
			}
			continue
		}
		rows = append(rows, res.rows...)
		if len(res.cached) > 0 {
			db.CacheIssues(res.repoName, res.cached)
		} else {
			// No issues — clear old cache for this repo.
			db.CacheIssues(res.repoName, nil)
		}
	}

	return rows, nil
}

// fetchRemoteTasksDirect fetches issues in parallel without caching (when DB is unavailable).
func fetchRemoteTasksDirect() ([]taskRow, error) {
	resolved, err := resolveForgeRepos()
	if err != nil {
		return nil, err
	}

	results := make([]repoResult, len(resolved))
	var wg sync.WaitGroup
	for i, r := range resolved {
		wg.Add(1)
		go func(idx int, r resolvedForgeRepo) {
			defer wg.Done()
			res := repoResult{repoName: r.repo.Name}
			issues, err := r.forge.ListIssues(context.Background(), r.owner, r.repoName, "open")
			if err != nil {
				res.err = err
			} else {
				for _, issue := range issues {
					res.rows = append(res.rows, taskRow{
						Task:   issueToTask(issue, r.repo.Name, r.forge.Type()),
						Source: string(r.forge.Type()),
					})
				}
			}
			results[idx] = res
		}(i, r)
	}
	wg.Wait()

	warned := map[string]bool{}
	var rows []taskRow
	for _, res := range results {
		if res.err != nil {
			msg := fmt.Sprintf("listing issues for %s: %v", res.repoName, res.err)
			if !warned[msg] {
				fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
				warned[msg] = true
			}
			continue
		}
		rows = append(rows, res.rows...)
	}

	return rows, nil
}

func issueToTask(issue forge.Issue, repoName string, ft forge.ForgeType) statedb.Task {
	meta, _ := json.Marshal(map[string]any{
		"url":    issue.URL,
		"labels": issue.Labels,
		"author": issue.Author,
	})
	return statedb.Task{
		ID:        forge.FormatRemoteID(ft, issue.Number),
		Title:     issue.Title,
		Description: issue.Body,
		Status:    issue.State,
		Repo:      repoName,
		CreatedAt: issue.CreatedAt,
		Metadata:  string(meta),
	}
}

func runTaskClaim(cmd *cobra.Command, args []string) error {
	id := args[0]

	ft, number, isRemote := forge.ParseRemoteID(id)
	if isRemote {
		return runTaskClaimRemote(ft, number)
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

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

func runTaskClaimRemote(ft forge.ForgeType, number int) error {
	if taskFlagRepo == "" {
		return fmt.Errorf("--repo is required when claiming remote issues")
	}

	_, f, owner, repo, err := resolveRepoForge(taskFlagRepo)
	if err != nil {
		return err
	}

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	body := fmt.Sprintf("Claimed by agent `%s`", taskFlagAgent)
	if err := f.CommentOnIssue(context.Background(), owner, repo, number, body); err != nil {
		return err
	}

	remoteID := forge.FormatRemoteID(ft, number)
	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task claim", map[string]string{
			"id":    remoteID,
			"agent": taskFlagAgent,
			"repo":  taskFlagRepo,
		})
		return output.New("json").Format(env)
	}

	fmt.Printf("Commented on %s: claimed by %s\n", remoteID, taskFlagAgent)
	return nil
}

func runTaskUpdate(cmd *cobra.Command, args []string) error {
	id := args[0]

	if _, _, isRemote := forge.ParseRemoteID(id); isRemote {
		return fmt.Errorf("cannot update remote issue status from mit; use gh/glab directly")
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

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
	id := args[0]

	ft, number, isRemote := forge.ParseRemoteID(id)
	if isRemote {
		return runTaskShowRemote(ft, number)
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	task, err := db.GetTask(id)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task show", task)
		return output.New("json").Format(env)
	}

	printTask(task)
	return nil
}

func runTaskShowRemote(ft forge.ForgeType, number int) error {
	if taskFlagRepo == "" {
		return fmt.Errorf("--repo is required when showing remote issues")
	}

	_, f, owner, repo, err := resolveRepoForge(taskFlagRepo)
	if err != nil {
		return err
	}

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	issue, err := f.GetIssue(context.Background(), owner, repo, number)
	if err != nil {
		return err
	}

	if getOutputFormat() == "json" {
		env := output.NewEnvelope("task show", issue)
		return output.New("json").Format(env)
	}

	remoteID := forge.FormatRemoteID(ft, number)
	fmt.Printf("ID:          %s\n", remoteID)
	fmt.Printf("Title:       %s\n", issue.Title)
	fmt.Printf("State:       %s\n", issue.State)
	fmt.Printf("URL:         %s\n", issue.URL)
	if issue.Author != "" {
		fmt.Printf("Author:      %s\n", issue.Author)
	}
	if issue.Body != "" {
		fmt.Printf("Description: %s\n", issue.Body)
	}
	if len(issue.Labels) > 0 {
		fmt.Printf("Labels:      %s\n", fmt.Sprint(issue.Labels))
	}
	fmt.Printf("Created:     %s\n", issue.CreatedAt.Format("2006-01-02 15:04:05"))
	return nil
}

func printTask(task *statedb.Task) {
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
}

// resolveRepoForge loads the workspace and resolves the forge for a given repo name.
func resolveRepoForge(repoName string) (*workspace.Workspace, forge.Forge, string, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, nil, "", "", err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return nil, nil, "", "", err
	}

	repo, err := ws.GetRepo(repoName)
	if err != nil {
		return nil, nil, "", "", err
	}

	f, owner, rName, err := forge.ResolveForge(repo.Name, repo.URL, repo.Forge)
	if err != nil {
		return nil, nil, "", "", err
	}

	if err := f.CheckAvailable(); err != nil {
		return nil, nil, "", "", err
	}

	return ws, f, owner, rName, nil
}

// shortID returns the first 8 characters of a UUID for display.
// Remote IDs (e.g., "github#42") are returned as-is.
func shortID(id string) string {
	if _, _, ok := forge.ParseRemoteID(id); ok {
		return id
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
