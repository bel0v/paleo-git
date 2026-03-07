#!/bin/bash
# Example external runner for paleo-git
#
# Counts import statements matching a pattern at a specific commit.
# Demonstrates the external runner contract:
#   - Receives PALEO_COMMIT and PALEO_REPO_PATH as env vars
#   - Prints a single JSON line to stdout: {"value": N, "files": [...]}
#
# Usage in paleo.yml:
#   runner:
#     exec: ["bash", "./examples/runners/count-imports.sh"]
#     config:
#       pattern: "from '@legacy/"

set -euo pipefail

# Parse config (passed as JSON in PALEO_RUNNER_CONFIG)
PATTERN=$(echo "$PALEO_RUNNER_CONFIG" | grep -o '"pattern":"[^"]*"' | cut -d'"' -f4)

if [ -z "$PATTERN" ]; then
  echo "Error: pattern not found in PALEO_RUNNER_CONFIG" >&2
  exit 1
fi

# Count matches using git grep
OUTPUT=$(git -C "$PALEO_REPO_PATH" grep -c "$PATTERN" "$PALEO_COMMIT" -- 2>/dev/null || true)

if [ -z "$OUTPUT" ]; then
  echo '{"value": 0}'
  exit 0
fi

# Parse git grep -c output: "commit:path:count"
TOTAL=0
FILES="[]"
FILE_LIST=""

while IFS= read -r line; do
  count="${line##*:}"
  # Extract file path (between first and last colon)
  path="${line#*:}"
  path="${path%:*}"
  TOTAL=$((TOTAL + count))
  if [ -z "$FILE_LIST" ]; then
    FILE_LIST="\"$path\""
  else
    FILE_LIST="$FILE_LIST, \"$path\""
  fi
done <<< "$OUTPUT"

echo "{\"value\": $TOTAL, \"files\": [$FILE_LIST]}"
