# paleo-git

Track code migration progress in git repositories.

paleo-git measures metrics (grep counts, custom scripts) at git commits and returns structured results. Use it to track how migrations progress over time — how many files still use the old pattern, which files they are, and whether things are getting better or worse.

## Install

```bash
# From source
go install github.com/bel0v/paleo-git/cmd/paleo-git@latest

# Homebrew
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
    "metric_hash": "a1b2c3d4e5f6...",
    "commit": "abc123...",
    "author_date": "2025-03-10T14:30:00Z",
    "value": 42,
    "files": ["src/old.ts", "src/legacy.ts"],
    "status": "ok",
    "duration_ms": 150
  }
]
```

3. Scan history and save results:

```bash
paleo-git scan --config paleo.yml --save-dir ./paleo-data
```

Streams one NDJSON line per (metric, commit) pair and saves results to a data directory. Resume a partial scan:

```bash
paleo-git scan --config paleo.yml --load-dir ./paleo-data --save-dir ./paleo-data
```

Already-measured (metric, config, commit) triples are skipped. If you change a metric's config (e.g. pattern), the new definition gets a different hash and all commits are re-measured for that metric.

## Commands

### `measure`

Run all metrics at a single commit.

```
paleo-git measure --config <file> [--commit <ref>] [--repo <path>] [--load-dir <dir>] [--save-dir <dir>] [--quiet]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | (required) | Path to YAML config |
| `--commit` | `HEAD` | Commit to measure |
| `--repo` | `.` | Path to git repository |
| `--load-dir` | (none) | Load prior results to skip already-measured metrics |
| `--save-dir` | (none) | Save new results to data directory |
| `--quiet` | `false` | Suppress stdout output |

Output: JSON array to stdout (unless `--quiet`). Only new results are printed — if `--load-dir` is set, already-measured results are excluded from output.

### `scan`

Traverse history and measure metrics at sampled commits.

```
paleo-git scan --config <file> [--repo <path>] [--load-dir <dir>] [--save-dir <dir>] [--quiet]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | (required) | Path to YAML config |
| `--repo` | `.` | Path to git repository |
| `--load-dir` | (none) | Load prior results to skip already-measured commits |
| `--save-dir` | (none) | Save results to data directory |
| `--quiet` | `false` | Suppress stdout output |

Output: NDJSON to stdout (one line per measurement, unless `--quiet`).

## Data directory

When using `--save-dir`, results are stored as NDJSON files organized by metric ID:

```
paleo-data/
  metrics/
    legacy-imports.jsonl
    todo-count.jsonl
```

Each line is a JSON object with the same schema as the stdout output. Files are append-only — new measurements are added to the end.

Use `--load-dir` to read existing results and skip re-measuring the same (metric_id, metric_hash, commit) triples. You can point both flags at the same directory for incremental scans.

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
  - id: <string>              # Unique identifier (no slashes or path separators)
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

Validation rules:
- Metric IDs must be unique
- Each metric must reference an existing traversal
- Runner must specify exactly one of `builtin` or `exec`
- `paths.include` must not be empty
- `sampling.every` must be at least 1

## Built-in runners

### `git_grep_count`

Counts lines matching a regex at a commit using `git grep -P` (Perl-compatible regex).

Config:
- `pattern` (required): Perl regex pattern to match

Pattern examples:
```yaml
# Simple string match
pattern: "from '@legacy/"

# Multiple patterns (alternation)
pattern: "TODO|FIXME|HACK"

# Word boundary (whole word only)
pattern: "\\bvar\\b"

# File extensions in imports
pattern: "\\.jsx?\\b"
```

Requires git compiled with PCRE support (default on macOS and most Linux distros).

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

## Architecture

paleo-git is a **stateless engine library** with a thin CLI wrapper. It does not persist results, compare values, or render dashboards — those are consumer concerns.

Designed to serve three consumers:
- **CLI** (this tool) — wraps the engine, outputs to stdout
- **GitHub Action** (separate repo) — CI integration with persistence and PR comments
- **Web Dashboard** (separate repo) — trends, file-level detail, migration overview

## License

Apache-2.0
