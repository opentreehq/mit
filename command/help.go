package command

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/urfave/cli/v3"
)

var guideTmpl = template.Must(template.New("guide").Parse(guideTemplate))

func GuideCommand(toolName string) *cli.Command {
	return &cli.Command{
		Name:  "guide",
		Usage: fmt.Sprintf("Comprehensive guide to %s (designed for LLM agents)", toolName),
		Action: func(_ context.Context, _ *cli.Command) error {
			var buf bytes.Buffer
			if err := guideTmpl.Execute(&buf, map[string]string{
				"Name": toolName,
			}); err != nil {
				return err
			}
			_, err := buf.WriteTo(os.Stdout)
			return err
		},
	}
}

const guideTemplate = `# {{.Name}} - Multi-repo Integration Tool

## Overview
{{.Name}} manages multiple repositories as a unified workspace without git submodules.
It supports both git and Sapling (sl) as VCS drivers and provides structured
JSON output for programmatic use by AI agents.

## Quick Start
1. Create a workspace: {{.Name}} init
2. Add repos: {{.Name}} add <url>
3. Clone all repos: {{.Name}} clone
4. Check status: {{.Name}} status

## Configuration ({{.Name}}.yaml)
The workspace is defined by a {{.Name}}.yaml file at the root:
` + "```yaml" + `
version: "1"
workspace:
  name: my-project
  description: "A multi-repo project"
repos:
  service-a:
    url: git@github.com:org/service-a.git
    branch: main
  service-b:
    url: git@github.com:org/service-b.git
    path: custom-path  # defaults to repo name
    branch: develop
` + "```" + `

## Core Commands

### Workspace Setup
- {{.Name}} init                    Create {{.Name}}.yaml in current directory
- {{.Name}} clone [--vcs git|sl]    Clone all repos (choose VCS driver)
- {{.Name}} add <url>               Add a repo to workspace and clone it
- {{.Name}} remove <name>           Remove a repo from {{.Name}}.yaml
- {{.Name}} list                    List all repos with metadata
- {{.Name}} doctor                  Validate workspace health

### VCS Operations
- {{.Name}} status                  Aggregated status of all repos
- {{.Name}} sync                    Pull latest default branch for all repos
- {{.Name}} pull                    Pull current branch for all repos
- {{.Name}} push                    Push repos with local commits
- {{.Name}} fetch                   Fetch all remotes in parallel
- {{.Name}} switch <branch>         Switch all repos to a branch (--create to create)
- {{.Name}} branch                  List current branches (--common for shared)
- {{.Name}} commit -m "msg"         Commit in all dirty repos (-a to stage all)
- {{.Name}} diff                    Show diffs across all repos
- {{.Name}} log [-n 10]             Interleaved commit log
- {{.Name}} grep <pattern>          Search across all repos

### Cross-Repo Execution
- {{.Name}} run <command>           Run a shell command in each repo directory

### Worktrees (for AI agents)
- {{.Name}} worktree create <name>  Create isolated worktrees across repos
- {{.Name}} worktree list           List active worktrees
- {{.Name}} worktree remove <name>  Clean up worktrees

## Multi-Agent Collaboration

### Tasks
Tasks coordinate work between multiple agents via SQLite (.{{.Name}}/state.db):
- {{.Name}} task create "title"           Create a new task
- {{.Name}} task list [--status open]     List tasks (filter by status, agent, repo)
- {{.Name}} task claim <id> --agent <id>  Atomically claim an open task
- {{.Name}} task update <id> --status done  Update task status
- {{.Name}} task show <id>                Show task details

Agent workflow:
1. {{.Name}} task list --status open --output json
2. {{.Name}} task claim <id> --agent <agent-id>
3. Do the work
4. {{.Name}} task update <id> --status done

### Memory
Persistent project knowledge in .{{.Name}}/memory/:
- {{.Name}} memory add "content"          Add a memory entry
- {{.Name}} memory list                   List all memories
- {{.Name}} memory search "query"         Search memories
- {{.Name}} memory remove <id>            Remove a memory

### Skills
Reusable workflows in .{{.Name}}/skills/:
- {{.Name}} skill list                    List available skills
- {{.Name}} skill show <name>             Show skill details
- {{.Name}} skill create <name>           Create a new skill
- {{.Name}} skill search "query"          Find relevant skills

## Semantic Search

### Indexing
Build embeddings index using qwen3-embedding (local, via llama.cpp):
- {{.Name}} index                   Build/update index incrementally
- {{.Name}} index --rebuild         Force full rebuild
- {{.Name}} index --status          Show index health

### Search
- {{.Name}} search "query"          Semantic search across all repos
- {{.Name}} search --limit 10 "q"   Control result count

## Context & Dependencies
- {{.Name}} context                 Generate workspace context document
- {{.Name}} deps                    Analyze inter-repo dependencies

## Agent-Specific Commands
- {{.Name}} discover                Full workspace topology as JSON
- {{.Name}} help --full             This guide
- {{.Name}} doctor --output json    Machine-readable health check

## Global Flags
All commands support these flags:
- --repos <names>     Filter to specific repos (comma-separated)
- --exclude <names>   Exclude specific repos
- -j <N>              Parallelism (default: num CPUs)
- --output json       JSON output (consistent envelope format)
- --output table      Human-readable table (default)
- --output plain      Plain text, one item per line
- -q, --quiet         Suppress progress output
- --dry-run           Show what would be done

## JSON Output Envelope
All --output json commands return:
` + "```json" + `
{
  "version": "1",
  "command": "status",
  "timestamp": "2026-03-26T10:00:00Z",
  "success": true,
  "results": [...],
  "summary": {...},
  "errors": []
}
` + "```" + `

## Exit Codes
- 0: Success
- 1: Partial failure (some repos failed)
- 2: Total failure or config error
- 3: No workspace found (no {{.Name}}.yaml)

## VCS Driver Notes
VCS is determined by --vcs on {{.Name}} clone, then auto-detected from .git/.sl for
all subsequent operations. A workspace can mix git and Sapling repos.

### Git: Standard git commands.
### Sapling: Uses sl clone, sl goto (checkout), sl pull (no merge), sl push --to.
  No staging area. Worktrees fall back to sl clone.
`
