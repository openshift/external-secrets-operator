#!/usr/bin/env bash
# Claude Code PostToolUse hook (matcher: Edit|Write).
#
# Auto-formats the file an agent just edited, so formatting is deterministic
# instead of relying on the agent remembering to run `make fmt`. Reads the
# hook JSON payload from stdin per the Claude Code hooks contract:
# https://code.claude.com/docs/en/hooks
set -euo pipefail

# If python3 is missing, the substitution below fails and (unlike the
# destructive-bash hook) there's no security property to fail closed for --
# just skip formatting silently rather than surfacing a raw tool error.
if ! command -v python3 >/dev/null 2>&1; then
  exit 0
fi

file_path=$(python3 -c '
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get("tool_input", {}).get("file_path", ""))
except Exception:
    print("")
')

if [ -z "$file_path" ] || [ ! -f "$file_path" ]; then
  exit 0
fi

case "$file_path" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file_path"
    ;;
  *.yaml|*.yml)
    # No-op: YAML formatting is intentionally left to the author/golangci-lint.
    ;;
esac

exit 0
