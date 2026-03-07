# paleo-git

Track code migration progress in git repositories.

paleo-git measures metrics (grep counts, custom scripts) at git commits and returns structured results. Use it to track how migrations progress over time — how many files still use the old pattern, which files they are, and whether things are getting better or worse.

## Install

```bash
# From source
go install github.com/bel0v/paleo-git/cmd/paleo-git@latest

# Homebrew (after first release)
brew install bel0v/tap/paleo-git

# Or download a binary from GitHub Releases
```

## Quick start

1. Create a config file `paleo.yml`:

```yaml
version: 1

traversals:
  default:
    range:
      start: "main~100"
      end: "HEAD"
    mode: first_parent
    sampling:
      every: 10

metrics:
  - id: legacy-imports
    traversal: default
    paths:
      include: ["src/**/*.ts"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "from '@legacy/"
```

2. Measure current state:

```bash
paleo-git measure --config paleo.yml
```

This prints a JSON array of results for HEAD:

```json
[
  {
    "metric_id": "legacy-imports",
    "commit": "abc123...",
    "value": 42,
    "files": ["src/old.ts", "src/legacy.ts"],
    "status": "ok",
    "duration_ms": 150
  }
]
```

3. Scan history:

```bash
paleo-git scan --config paleo.yml > results.jsonl
```

Streams one NDJSON line per (metric, commit) pair. Resume a scan:

```bash
paleo-git scan --config paleo.yml --skip results.jsonl >> results.jsonl
```

## Commands

### `measure`

Run all metrics at a single commit.

```
paleo-git measure --config <file> [--commit <ref>] [--repo <path>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | (required) | Path to YAML config |
| `--commit` | `HEAD` | Commit to measure |
| `--repo` | `.` | Path to git repository |

Output: JSON array to stdout.

### `scan`

Traverse history and measure metrics at sampled commits.

```
paleo-git scan --config <file> [--skip <file>] [--repo <path>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | (required) | Path to YAML config |
| `--skip` | (none) | NDJSON file of prior results to skip |
| `--repo` | `.` | Path to git repository |

Output: NDJSON to stdout (one line per measurement).

## Config reference

```yaml
version: 1                    # Config schema version

traversals:
  <name>:                     # Named traversal (referenced by metrics)
    range:
      start: "main~500"      # Start ref (exclusive)
      end: "HEAD"             # End ref (inclusive)
    mode: first_parent        # Traversal mode (first_parent only for now)
    sampling:
      every: 25               # Stride: 1 = every commit, 10 = every 10th

metrics:
  - id: <string>              # Unique identifier
    description: <string>     # Optional human description
    traversal: <name>         # Required: which traversal to use
    paths:
      include: [<globs>]      # Required: file patterns to search
      exclude: [<globs>]      # Optional: patterns to exclude
    runner:
      builtin: git_grep_count # OR exec: [<command>, <args>...]
      config:                  # Runner-specific config (opaque)
        pattern: "..."
```

## Built-in runners

### `git_grep_count`

Counts lines matching a regex at a commit using `git grep`.

Config:
- `pattern` (required): regex pattern to match

Returns: match count and list of files with matches.

## Writing external runners

An external runner is any executable that:

1. Reads environment variables:
   - `PALEO_COMMIT` — full commit SHA
   - `PALEO_REPO_PATH` — absolute path to the repo
   - `PALEO_RUNNER_CONFIG` — JSON string of runner config (from YAML)
   - `PALEO_PATHS_INCLUDE` — JSON array of include globs
   - `PALEO_PATHS_EXCLUDE` — JSON array of exclude globs

2. Prints a single JSON line to stdout:

```json
{"value": 42, "files": ["src/old.ts", "src/legacy.ts"]}
```

`value` is required (integer). `files` is optional.

Exit code 0 = success. Non-zero = error (stderr is captured).

See `examples/runners/count-imports.sh` for a working example.

## Architecture

paleo-git is a **stateless engine library** with a thin CLI wrapper. It does not persist results, compare values, or render dashboards — those are consumer concerns.

Designed to serve three consumers:
- **CLI** (this tool) — wraps the engine, outputs to stdout
- **GitHub Action** (separate repo) — CI integration with persistence and PR comments
- **Web Dashboard** (separate repo) — trends, file-level detail, migration overview

## License

Apache-2.0
