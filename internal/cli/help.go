package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var guideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Comprehensive guide to mit (designed for LLM agents)",
	RunE:  runGuide,
}

func init() {
	rootCmd.AddCommand(guideCmd)
}

func runGuide(cmd *cobra.Command, args []string) error {
	fmt.Fprint(os.Stdout, fullHelp)
	return nil
}

const fullHelp = `# MIT - Multi-repo Integration Tool

## Overview
mit manages multiple repositories as a unified workspace without git submodules.
It supports both git and Sapling (sl) as VCS drivers and provides structured
JSON output for programmatic use by AI agents.

## Quick Start
1. Create a workspace: mit init
2. Add repos: mit add <url>
3. Clone all repos: mit clone
4. Check status: mit status

## Configuration (mit.yaml)
The workspace is defined by a mit.yaml file at the root:
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
- mit init                    Create mit.yaml in current directory
- mit clone [--vcs git|sl]    Clone all repos (choose VCS driver)
- mit add <url>               Add a repo to workspace and clone it
- mit remove <name>           Remove a repo from mit.yaml
- mit list                    List all repos with metadata
- mit doctor                  Validate workspace health

### VCS Operations
- mit status                  Aggregated status of all repos
- mit sync                    Pull latest default branch for all repos
- mit pull                    Pull current branch for all repos
- mit push                    Push repos with local commits
- mit fetch                   Fetch all remotes in parallel
- mit switch <branch>         Switch all repos to a branch (--create to create)
- mit branch                  List current branches (--common for shared)
- mit commit -m "msg"         Commit in all dirty repos (-a to stage all)
- mit diff                    Show diffs across all repos
- mit log [-n 10]             Interleaved commit log
- mit grep <pattern>          Search across all repos

### Cross-Repo Execution
- mit run <command>           Run a shell command in each repo directory

### Worktrees (for AI agents)
- mit worktree create <name>  Create isolated worktrees across repos
- mit worktree list           List active worktrees
- mit worktree remove <name>  Clean up worktrees

## Multi-Agent Collaboration

### Tasks
Tasks coordinate work between multiple agents via SQLite (.mit/state.db):
- mit task create "title"           Create a new task
- mit task list [--status open]     List tasks (filter by status, agent, repo)
- mit task claim <id> --agent <id>  Atomically claim an open task
- mit task update <id> --status done  Update task status
- mit task show <id>                Show task details

Agent workflow:
1. mit task list --status open --output json
2. mit task claim <id> --agent <agent-id>
3. Do the work
4. mit task update <id> --status done

### Memory
Persistent project knowledge in .mit/memory/:
- mit memory add "content"          Add a memory entry
- mit memory list                   List all memories
- mit memory search "query"         Search memories
- mit memory remove <id>            Remove a memory

### Skills
Reusable workflows in .mit/skills/:
- mit skill list                    List available skills
- mit skill show <name>             Show skill details
- mit skill create <name>           Create a new skill
- mit skill search "query"          Find relevant skills

## Semantic Search

### Indexing
Build embeddings index using qwen3-embedding (local, via llama.cpp):
- mit index                   Build/update index incrementally
- mit index --rebuild         Force full rebuild
- mit index --status          Show index health

### Search
- mit search "query"          Semantic search across all repos
- mit search --limit 10 "q"   Control result count

## Context & Dependencies
- mit context                 Generate workspace context document
- mit deps                    Analyze inter-repo dependencies

## Agent-Specific Commands
- mit discover                Full workspace topology as JSON
- mit help --full             This guide
- mit doctor --output json    Machine-readable health check

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
- 3: No workspace found (no mit.yaml)

## VCS Driver Notes
VCS is determined by --vcs on mit clone, then auto-detected from .git/.sl for
all subsequent operations. A workspace can mix git and Sapling repos.

### Git: Standard git commands.
### Sapling: Uses sl clone, sl goto (checkout), sl pull (no merge), sl push --to.
  No staging area. Worktrees fall back to sl clone.
`
