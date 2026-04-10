package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gabemeola/mit/forge"
	"github.com/gabemeola/mit/output"
	"github.com/gabemeola/mit/statedb"
	"github.com/gabemeola/mit/workspace"
	"github.com/urfave/cli/v3"
)

// remoteListOpts carries flag values for remote task listing without package-level mutable state.
type remoteListOpts struct {
	RepoFilter   string
	Refresh      bool
	OutputFormat string
}

// TaskCommand returns the task command with create, list, claim, update, and show subcommands.
func TaskCommand() *cli.Command {
	return &cli.Command{
		Name:        "task",
		Usage:       "Manage tasks for agents and humans",
		Description: "Manage tasks for agents and humans.",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create a new task",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "description", Usage: "task description", Local: true},
					&cli.StringFlag{Name: "repo", Usage: "associated repo", Local: true},
					&cli.BoolFlag{Name: "remote", Usage: "create issue on forge instead of locally", Local: true},
				},
				Action: taskCreateAction,
			},
			{
				Name:  "list",
				Usage: "List tasks",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "status", Usage: "filter by status", Local: true},
					&cli.StringFlag{Name: "agent", Usage: "filter by agent id", Local: true},
					&cli.StringFlag{Name: "repo", Usage: "filter by repo", Local: true},
					&cli.StringFlag{Name: "source", Value: "all", Usage: "task source: local, remote, all", Local: true},
					&cli.BoolFlag{Name: "refresh", Usage: "force refresh of cached remote issues", Local: true},
				},
				Action: taskListAction,
			},
			{
				Name:  "claim",
				Usage: "Claim a task for an agent",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "agent", Usage: "agent id to claim as", Required: true, Local: true},
					&cli.StringFlag{Name: "repo", Usage: "repo for remote issues", Local: true},
				},
				Action: taskClaimAction,
			},
			{
				Name:  "update",
				Usage: "Update a task's status",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "status", Usage: "new status", Required: true, Local: true},
				},
				Action: taskUpdateAction,
			},
			{
				Name:  "show",
				Usage: "Show details of a task",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "repo", Usage: "repo for remote issues", Local: true},
				},
				Action: taskShowAction,
			},
		},
	}
}

func openStateDB() (*statedb.DB, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return statedb.Open(dir)
}

func taskCreateAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	title := cmd.Args().First()

	if cmd.Bool("remote") {
		return runTaskCreateRemote(title, cmd.String("description"), cmd.String("repo"), getOutputFormat(cmd))
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	id, err := db.CreateTask(title, cmd.String("description"), cmd.String("repo"))
	if err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
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

func runTaskCreateRemote(title, description, repo, outputFormat string) error {
	if repo == "" {
		return fmt.Errorf("--repo is required when creating remote issues")
	}

	ws, f, owner, repoName, err := resolveRepoForge(repo)
	if err != nil {
		return err
	}
	_ = ws

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	issue, err := f.CreateIssue(context.Background(), owner, repoName, title, description)
	if err != nil {
		return err
	}

	if outputFormat == "json" {
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

func taskListAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return fmt.Errorf("expected 0 argument(s)")
	}

	source := cmd.String("source")
	status := cmd.String("status")
	agent := cmd.String("agent")
	repo := cmd.String("repo")
	remoteOpts := remoteListOpts{
		RepoFilter:   repo,
		Refresh:      cmd.Bool("refresh"),
		OutputFormat: getOutputFormat(cmd),
	}

	var rows []taskRow

	// Local tasks
	if source == "local" || source == "all" {
		db, err := openStateDB()
		if err != nil {
			if source == "local" {
				return err
			}
			fmt.Fprintf(os.Stderr, "warning: could not open local task db: %v\n", err)
		} else {
			defer db.Close()
			tasks, err := db.ListTasks(status, agent, repo)
			if err != nil {
				return err
			}
			for _, t := range tasks {
				rows = append(rows, taskRow{Task: t, Source: "local"})
			}
		}
	}

	// Remote tasks — stale-while-revalidate.
	if source == "remote" || source == "all" {
		remote, err := getRemoteTasks(remoteOpts)
		if err != nil {
			if source == "remote" {
				return err
			}
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		rows = append(rows, remote...)
	}

	if getOutputFormat(cmd) == "json" {
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
func getRemoteTasks(opts remoteListOpts) ([]taskRow, error) {
	db, err := openStateDB()
	if err != nil {
		// Can't cache — fall back to direct fetch.
		return fetchRemoteTasksDirect(opts.RepoFilter)
	}

	// Try serving from cache.
	if !opts.Refresh {
		cached, err := db.GetCachedIssues(opts.RepoFilter)
		if err == nil && len(cached) > 0 {
			// Serve cached data immediately.
			rows := cachedToRows(cached)

			// If stale, kick off background refresh for next run.
			cacheAge := db.GetCacheAge(nil)
			if cacheAge <= 0 || cacheAge > 5*time.Minute {
				outFmt := opts.OutputFormat
				repoF := opts.RepoFilter
				go func() {
					bgDB, err := openStateDB()
					if err != nil {
						return
					}
					defer bgDB.Close()
					_, _ = fetchAndCacheRemoteTasks(bgDB, outFmt, repoF)
				}()
			}

			db.Close()
			return rows, nil
		}
	}

	// No cache — fetch synchronously (first run or --refresh).
	rows, err := fetchAndCacheRemoteTasks(db, opts.OutputFormat, opts.RepoFilter)
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
				ID:          ci.ID,
				Title:       ci.Title,
				Description: ci.Body,
				Status:      ci.Status,
				Repo:        ci.Repo,
				CreatedAt:   ci.CreatedAt,
				Metadata:    ci.URL,
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
func resolveForgeRepos(repoFilter string) ([]resolvedForgeRepo, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	ws, err := workspace.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}

	sel := workspace.NewSelector(repoFilter, "")
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
			if repoFilter != "" {
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
func fetchAndCacheRemoteTasks(db *statedb.DB, outputFormat string, repoFilter string) ([]taskRow, error) {
	resolved, err := resolveForgeRepos(repoFilter)
	if err != nil {
		return nil, err
	}

	if len(resolved) == 0 {
		return nil, nil
	}

	showProgress := outputFormat != "json"
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
func fetchRemoteTasksDirect(repoFilter string) ([]taskRow, error) {
	resolved, err := resolveForgeRepos(repoFilter)
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
		ID:          forge.FormatRemoteID(ft, issue.Number),
		Title:       issue.Title,
		Description: issue.Body,
		Status:      issue.State,
		Repo:        repoName,
		CreatedAt:   issue.CreatedAt,
		Metadata:    string(meta),
	}
}

func taskClaimAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	id := cmd.Args().First()
	agent := cmd.String("agent")
	repo := cmd.String("repo")

	ft, number, isRemote := forge.ParseRemoteID(id)
	if isRemote {
		return runTaskClaimRemote(ft, number, agent, repo, getOutputFormat(cmd))
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.ClaimTask(id, agent); err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		task, err := db.GetTask(id)
		if err != nil {
			return err
		}
		env := output.NewEnvelope("task claim", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("Task %s claimed by %s\n", shortID(id), agent)
	return nil
}

func runTaskClaimRemote(ft forge.ForgeType, number int, agent, repo, outputFormat string) error {
	if repo == "" {
		return fmt.Errorf("--repo is required when claiming remote issues")
	}

	_, f, owner, repoName, err := resolveRepoForge(repo)
	if err != nil {
		return err
	}

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	body := fmt.Sprintf("Claimed by agent `%s`", agent)
	if err := f.CommentOnIssue(context.Background(), owner, repoName, number, body); err != nil {
		return err
	}

	remoteID := forge.FormatRemoteID(ft, number)
	if outputFormat == "json" {
		env := output.NewEnvelope("task claim", map[string]string{
			"id":    remoteID,
			"agent": agent,
			"repo":  repo,
		})
		return output.New("json").Format(env)
	}

	fmt.Printf("Commented on %s: claimed by %s\n", remoteID, agent)
	return nil
}

func taskUpdateAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	id := cmd.Args().First()
	status := cmd.String("status")

	if _, _, isRemote := forge.ParseRemoteID(id); isRemote {
		return fmt.Errorf("cannot update remote issue status from mit; use gh/glab directly")
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.UpdateTaskStatus(id, status); err != nil {
		return err
	}

	if getOutputFormat(cmd) == "json" {
		task, err := db.GetTask(id)
		if err != nil {
			return err
		}
		env := output.NewEnvelope("task update", task)
		return output.New("json").Format(env)
	}

	fmt.Printf("Task %s status updated to %s\n", shortID(id), status)
	return nil
}

func taskShowAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("expected 1 argument(s)")
	}
	id := cmd.Args().First()
	repo := cmd.String("repo")

	ft, number, isRemote := forge.ParseRemoteID(id)
	if isRemote {
		return runTaskShowRemote(ft, number, repo, getOutputFormat(cmd))
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

	if getOutputFormat(cmd) == "json" {
		env := output.NewEnvelope("task show", task)
		return output.New("json").Format(env)
	}

	printTask(task)
	return nil
}

func runTaskShowRemote(ft forge.ForgeType, number int, repo, outputFormat string) error {
	if repo == "" {
		return fmt.Errorf("--repo is required when showing remote issues")
	}

	_, f, owner, repoName, err := resolveRepoForge(repo)
	if err != nil {
		return err
	}

	if err := f.CheckAuthenticated(); err != nil {
		return err
	}

	issue, err := f.GetIssue(context.Background(), owner, repoName, number)
	if err != nil {
		return err
	}

	if outputFormat == "json" {
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
