# mit

Multi-repo Integration Tool. Manage multiple repositories as a unified workspace without git submodules. Supports both Git and Sapling as VCS drivers. Designed for both humans and AI agents.

## Install


### With Homebrew

```sh
brew install opentreehq/tap/mit
```

## Usage

```sh
# Initialize a new workspace
mit init

# Clone all repos defined in mit.yaml
mit clone

# Check workspace status
mit status

# Pull latest on all repos
mit sync

# Run a command across all repos
mit run "git log --oneline -5"

# Semantic search (requires full build)
mit index
mit search "rate limiting logic"
```

### Configuration

A workspace is defined by a `mit.yaml` file at the root:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/gabemeola/mit/refs/heads/main/mit.schema.json
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
```

### Commands

#### Workspace Setup
| Command | Description |
|---------|-------------|
| `mit init` | Create `mit.yaml` in current directory |
| `mit clone [--vcs git\|sl]` | Clone all repos |
| `mit add <url>` | Add a repo and clone it |
| `mit remove <name>` | Remove a repo from `mit.yaml` |
| `mit list` | List all repos with metadata |
| `mit doctor` | Validate workspace health |

#### VCS Operations
| Command | Description |
|---------|-------------|
| `mit status` | Aggregated status of all repos |
| `mit sync` | Pull latest default branch for all repos |
| `mit pull` | Pull current branch for all repos |
| `mit push` | Push repos with local commits |
| `mit fetch` | Fetch all remotes in parallel |
| `mit switch <branch>` | Switch branches (`--create` to create) |
| `mit branch` | List branches (`--common` for shared) |
| `mit commit -m "msg"` | Commit in all dirty repos |
| `mit diff` | Show diffs across all repos |
| `mit log` | Interleaved commit log |
| `mit grep <pattern>` | Search across all repos |
| `mit run <cmd>` | Run a shell command in each repo |

#### Global Flags
| Flag | Description |
|------|-------------|
| `--repos <names>` | Filter to specific repos (comma-separated) |
| `--exclude <names>` | Exclude specific repos |
| `-j <N>` | Parallelism (default: num CPUs) |
| `--output json\|table\|plain` | Output format |
| `-q, --quiet` | Suppress progress output |
| `--dry-run` | Show what would be done |

## Using mit with AI Agents

mit is designed to be used by AI agents for navigating and understanding large multi-repo codebases. All commands support `--output json` for structured output.

### Semantic Index and Search

mit includes a local semantic search engine powered by [llama.cpp](https://github.com/ggml-org/llama.cpp) and the qwen3-embedding model. Embeddings are computed entirely on-device -- no API calls or external services.

#### Building the Index

```sh
# Build the index (incremental -- only re-indexes changed files)
mit index

# Force a full rebuild
mit index --rebuild

# Check index health
mit index --status
```

On first run, `mit index` downloads the embedding model (~400MB) to `~/.mit/models/` and indexes all files across every repo in the workspace. Subsequent runs are incremental -- only files that changed since the last index are re-embedded.

The index is stored in `.mit/state.db` (SQLite). Files are split into ~50-line chunks that respect function/class boundaries, and each chunk is embedded into a 1024-dimensional vector.

#### Searching

```sh
# Semantic search across all repos
mit search "rate limiting middleware"

# Limit results
mit search --limit 5 "database connection pooling"

# Show matching source code inline
mit search --content "error handling in auth"

# JSON output for programmatic use
mit search --output json "retry logic"
```

Results include the repo name, file path, line range, and a relevance score:

```
web-api:src/middleware/rateLimit.ts:1-50 (0.847)
engine:lib/throttle/limiter.go:23-71 (0.812)
```

With `--content`, the matching source lines are displayed inline beneath each result.

#### JSON Output

```sh
mit search --output json "query"
```

```json
{
  "version": "1",
  "command": "search",
  "timestamp": "2026-03-27T10:00:00Z",
  "success": true,
  "results": [
    {
      "repo": "web-api",
      "file": "src/middleware/rateLimit.ts",
      "line_start": 1,
      "line_end": 50,
      "score": 0.847
    }
  ],
  "summary": { "matches": 2 }
}
```

#### Agent Workflow Example

An agent can use `mit index` + `mit search` to quickly locate relevant code across a large workspace before making changes:

```sh
# 1. Discover the workspace layout
mit discover

# 2. Build/update the semantic index
mit index

# 3. Find relevant code
mit search --output json --limit 10 "authentication flow"

# 4. Read the matching files and make changes
# ...

# 5. Verify workspace state
mit status --output json
```

### Worktrees

Worktrees let agents work in isolated copies of every repo without affecting the main checkout:

```sh
# Create worktrees across all repos
mit worktree create feature-auth

# List active worktrees
mit worktree list

# Clean up when done
mit worktree remove feature-auth
```

For Git repos this uses `git worktree add` (fast, shares object store). For Sapling repos it falls back to `sl clone`.

### Task Coordination

Multiple agents can coordinate work through mit's task system, backed by SQLite with WAL mode for safe concurrent access:

```sh
# Create tasks
mit task create "Refactor auth middleware" --repo web-api

# Agent discovers available work
mit task list --status open --output json

# Agent atomically claims a task (fails if already claimed)
mit task claim <id> --agent agent-1

# Agent marks work as done
mit task update <id> --status done
```

### Memory and Skills

Agents can persist knowledge and discover reusable workflows:

```sh
# Store an observation
mit memory add "web-api uses RabbitMQ for async messaging to the scheduler"

# Search memories
mit memory search "messaging"

# Discover available skills
mit skill list
mit skill show run-backend-tests
```

### Agent-Specific Commands

| Command | Description |
|---------|-------------|
| `mit discover` | Full workspace topology as JSON |
| `mit guide` | Comprehensive guide to all mit features (designed for LLM consumption) |
| `mit context` | Auto-generate workspace context document |
| `mit deps` | Analyze inter-repo dependencies |
| `mit doctor --output json` | Machine-readable health check |

## Build Targets

All binaries are output to `./dist/`.

| Task | Platform | Output |
|------|----------|--------|
| `task build` | Auto-detect | `dist/mit` |
| `task build:macos-arm64` | macOS Apple Silicon | `dist/mit` |
| `task build:linux-amd64` | Linux x86_64 | `dist/mit-linux-amd64` |
| `task build:linux-arm64` | Linux aarch64 | `dist/mit-linux-arm64` |
| `task build:windows-amd64` | Windows x86_64 | `dist/mit-windows-amd64.exe` |
| `task build-lite` | Any (no CGo) | `dist/mit` |

Cross-compilation from macOS uses Zig as the C/C++ toolchain. Linux and Windows targets are CPU-only; macOS uses Metal + Accelerate.

## Contributing

### Prerequisites

- [Go](https://go.dev/) 1.26+
- [Task](https://taskfile.dev/) (task runner)
- [CMake](https://cmake.org/) (for building llama.cpp)
- [Zig](https://ziglang.org/) (only needed for cross-compilation)

### Build from source  

```sh
# Clone with submodules
git clone --recurse-submodules https://github.com/opentreehq/mit.git
cd mit

# Build for your current platform
task build

# Or install to $GOPATH/bin
task install
```

#### Lite Build (no embedding support)

If you don't need `mit index` / `mit search`, you can build without CGo or llama.cpp:

```sh
task build-lite
```

This produces a smaller binary without the `index` and `search` commands.

### Development

```sh
# Run tests
task test

# Run tests with verbose output
task test-verbose

# Run tests with coverage
task test-coverage

# Lint
task lint

# Clean all build artifacts
task clean
```

### Project Structure

```
mit/
├── cmd/mit/              # Entry point
├── internal/
│   ├── cli/              # Cobra commands
│   ├── config/           # mit.yaml parsing + schema
│   ├── vcs/              # Git and Sapling drivers
│   ├── workspace/        # Workspace loading + repo filtering
│   ├── executor/         # Parallel execution engine
│   ├── index/            # Code chunking + embedding orchestration
│   ├── embedding/        # CGo wrapper around llama.cpp
│   │   └── llama/        # llama.cpp bindings + platform-specific CGo flags
│   ├── statedb/          # SQLite state store (tasks, index, checksums)
│   ├── memory/           # .mit/memory/ markdown store
│   ├── skills/           # .mit/skills/ discovery
│   └── output/           # JSON, table, plain formatters
├── cmake/                # Zig cross-compilation toolchain files
├── third_party/
│   └── llama.cpp/        # Vendored llama.cpp (git submodule)
└── Taskfile.yml
```

### Build Tags

- **Default build**: includes llama.cpp embedding support via CGo. Requires CMake and the llama.cpp submodule.
- **`noembed`**: excludes all embedding code. No CGo required. `mit index` and `mit search` commands are omitted from the binary entirely.

### Adding a New Command

1. Create `internal/cli/<command>.go`
2. Define the cobra command and register it in `init()` with `rootCmd.AddCommand()`
3. If the command depends on embeddings, add `//go:build !noembed` at the top

### Cross-Compilation

Cross-compilation uses Zig as a drop-in C/C++ compiler via CMake toolchain files in `cmake/`. Each target has:

- A CMake toolchain file (e.g., `cmake/zig-linux-x86_64.cmake`)
- Wrapper scripts for `zig cc`/`zig c++` (required because CMake needs a single executable path)
- Platform-specific CGo linker flags in `internal/embedding/llama/cgo_<platform>.go`

To add a new cross-compilation target:

1. Add wrapper scripts in `cmake/` for the new zig target triple
2. Add a CMake toolchain file referencing those wrappers
3. Add `build-llama:<os>-<arch>` and `build:<os>-<arch>` tasks to `Taskfile.yml`
4. Add a `cgo_<platform>.go` file with the appropriate `#cgo LDFLAGS`

## License

[MPL-2.0](LICENSE)
