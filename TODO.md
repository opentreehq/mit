# MIT - Future Features

## Deferred Commands

### mit snapshot / mit restore
Capture exact commit SHAs across all repos for reproducible workspace state.
Useful for bug reports: "clone this workspace at these exact commits."

### mit stash / mit stash pop
Cross-repo stash. Stash changes in all dirty repos before a branch switch,
restore them afterward.

### mit pr
Create pull requests across multiple repos simultaneously.
Integrates with `gh` (GitHub) and `glab` (GitLab) CLIs.

### mit ci
Check CI/CD pipeline status across all repos.
"Is everything green?" at a glance.

### mit blame <pattern>
Cross-repo blame: find who last touched code matching a pattern
across the entire workspace.

### mit clean
Remove build artifacts (node_modules, dist, build, __pycache__, etc.)
across all repos. Free disk space.

## Enhancements

### Group support
Named sets of repos (backend, frontend, infra) for filtering operations.
`mit status --group backend`

### Hooks
Lifecycle hooks (post-clone, post-sync, pre-switch, post-switch)
for running commands automatically.

### Import from existing
- `mit init --from-gitmodules` — import from .gitmodules
- `mit init --from-meta` — import from .meta (meta tool)

### Agent schema export
`mit agent-schema` — output tool/function definitions compatible with
Claude, OpenAI, and other agent frameworks.

### Dependency-aware operations
Topological sort for build/install based on `depends_on` in mit.yaml.
Build dependencies before dependents.
