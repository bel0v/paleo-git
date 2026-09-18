#!/usr/bin/env python3
"""Example external runner for paleo-git.

Counts lines matching a regex at a commit, honouring the metric's paths, and
demonstrates the runner contract:

  reads   PALEO_COMMIT, PALEO_REPO_PATH, PALEO_RUNNER_CONFIG,
          PALEO_PATHS_INCLUDE, PALEO_PATHS_EXCLUDE   (JSON where noted)
  prints  one JSON object: {"value": <int>, "files": [<path>, ...]}
  exits   0 on success; any other code is a measurement failure and stderr
          is captured into the result's error

Usage in paleo.yml:

  runner:
    exec: ["python3", "examples/runners/count_imports.py"]
    config:
      pattern: "from '@legacy/"
"""
import json
import os
import subprocess
import sys

config = json.loads(os.environ.get("PALEO_RUNNER_CONFIG", "{}"))
pattern = config.get("pattern")
if not pattern:
    sys.exit("count_imports: config.pattern is required")

include = json.loads(os.environ.get("PALEO_PATHS_INCLUDE", "[]"))
exclude = json.loads(os.environ.get("PALEO_PATHS_EXCLUDE", "[]"))

# Same pathspec handling as the built-in runners: includes verbatim, excludes
# with git's ":!" magic. "-z" keeps paths containing ":" unambiguous.
cmd = ["git", "-C", os.environ["PALEO_REPO_PATH"], "grep", "-P", "-c", "-z", "-e", pattern,
       os.environ["PALEO_COMMIT"], "--", *include, *(f":!{path}" for path in exclude)]
result = subprocess.run(cmd, capture_output=True, text=True)
if result.returncode not in (0, 1):  # 1 means no matches
    sys.exit(result.stderr.strip() or f"git grep exited {result.returncode}")

total, files = 0, []
for line in result.stdout.splitlines():
    if not line:
        continue
    path, count = line.rsplit("\0", 1)
    total += int(count)
    files.append(path.split(":", 1)[1])

print(json.dumps({"value": total, "files": files}))
