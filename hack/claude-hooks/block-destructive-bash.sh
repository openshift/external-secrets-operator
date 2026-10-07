#!/usr/bin/env bash
# Claude Code PreToolUse hook (matcher: Bash).
#
# Blocks obviously destructive shell commands before they execute. This is a
# deterministic backstop, not a replacement for review -- it only catches
# unambiguous footguns (recursive deletes, force pushes/resets, raw device
# writes, fork bombs). Exit 2 blocks the tool call; exit 0 allows it.
# Hook contract: https://code.claude.com/docs/en/hooks
set -euo pipefail

# If python3 is missing, the command substitution below would fail with
# exit 127 (not 2). Claude Code only treats exit 2 as "block" -- any other
# non-zero exit is a non-blocking tool error, so the Bash command would run
# anyway. Fail closed instead: block when we can't even parse the input.
if ! command -v python3 >/dev/null 2>&1; then
  echo "Blocked: python3 not found, cannot inspect command for safety" >&2
  exit 2
fi

command=$(python3 -c '
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get("tool_input", {}).get("command", ""))
except Exception:
    print("")
')

if [ -z "$command" ]; then
  exit 0
fi

block() {
  # Deliberately do not echo $command: a blocked command may itself contain
  # secrets/tokens/hostnames, and that text would otherwise land in hook
  # output/session logs. Only the fixed category label is printed.
  echo "Blocked potentially destructive command: $1" >&2
  exit 2
}

# A flag/subcommand token boundary is whitespace, end-of-string, or a shell
# metacharacter that can immediately follow it (`;`, `&`, `|`, `<`, `>`, a
# backtick, or `$` starting a `$(...)` substitution) -- e.g.
# `git clean -fd; echo done` and `git clean -fd$(true)` both have no space
# before what follows the flags.
B='[[:space:];&|<>`$]'
# `git[[:space:]].*<subcommand>` (not `git[[:space:]]+<subcommand>`): git
# accepts global options before the subcommand (`git -C . reset --hard`,
# `git --git-dir=.git push --force`), so require only that the subcommand
# appears somewhere after "git ", not immediately adjacent to it.
# Block-device path fragment shared by the redirect/dd/tee checks below --
# covers SATA/SCSI (sdX), old IDE (hdX), virtio (vdX, the common disk name in
# KVM/cloud VMs), Xen (xvdX), NVMe (nvme0n1), and SD/eMMC (mmcblk0) naming.
DEV='(/dev/(sd|hd|vd|xvd)[a-z]|/dev/nvme[0-9]+n[0-9]+|/dev/mmcblk[0-9]+)'
# --- Single-token flag forms (-rf, -fr, -fd, -df, ...) ----------------------
deny_patterns=(
  "rm[[:space:]]+(-[A-Za-z]*[rR][A-Za-z]*f|-[A-Za-z]*f[A-Za-z]*[rR])($B|\$)"
  "git[[:space:]].*push[[:space:]]+.*(--force|-f|--mirror)($B|\$)"
  # `git push origin +HEAD:main` force-updates main via a "+"-prefixed
  # refspec -- no --force/-f flag appears at all, so it needs its own check.
  'git[[:space:]].*push[[:space:]]+.*[[:space:]]\+[^[:space:]]'
  'git[[:space:]].*reset[[:space:]]+--hard'
  "git[[:space:]].*clean[[:space:]]+(-[A-Za-z]*[fF][A-Za-z]*[dD]|-[A-Za-z]*[dD][A-Za-z]*[fF])($B|\$)"
  ">[[:space:]]*$DEV"
  "dd[[:space:]]+.*of=$DEV"
  # tee writes to its FILE operands directly (no "of=" prefix, no leading
  # ">"), so it's a distinct bypass of both the redirect and dd patterns
  # above -- e.g. `echo bad | tee /dev/sda`.
  "tee[[:space:]]+.*$DEV"
  # mkfs's own frontend is bare "mkfs -t <type> <dev>" (space-separated);
  # "mkfs.<type>" (e.g. mkfs.ext4) is a filesystem-specific symlink to it --
  # match both forms instead of only the dotted one.
  'mkfs[.[:space:]]'
  ':\(\)[[:space:]]*\{[[:space:]]*:\|:[[:space:]]*&[[:space:]]*\}[[:space:]]*;'
)
deny_labels=(
  "rm -rf/-fr"
  "git push --force/--mirror"
  "git push +refspec (force)"
  "git reset --hard"
  "git clean -fd/-df"
  "raw write to block device"
  "dd write to block device"
  "tee write to block device"
  "mkfs"
  "fork bomb"
)

for i in "${!deny_patterns[@]}"; do
  if [[ "$command" =~ ${deny_patterns[$i]} ]]; then
    block "${deny_labels[$i]}"
  fi
done

# --- Separated/long-form flags (`rm -r -f`, `rm --recursive --force`, ------
# --- `git clean -f -d`) that a single combined-flag regex can't express ----
if [[ "$command" =~ (^|[[:space:]])rm[[:space:]] ]]; then
  rm_recursive=0
  rm_force=0
  [[ "$command" =~ (^|[[:space:]])(-[A-Za-z]*[rR][A-Za-z]*|--recursive)($B|$) ]] && rm_recursive=1
  [[ "$command" =~ (^|[[:space:]])(-[A-Za-z]*f[A-Za-z]*|--force)($B|$) ]] && rm_force=1
  if [[ $rm_recursive -eq 1 && $rm_force -eq 1 ]]; then
    block "rm --recursive --force (separate flags)"
  fi
fi

if [[ "$command" =~ git[[:space:]].*clean[[:space:]] ]]; then
  clean_force=0
  clean_dirs=0
  # [A-Za-z]* on both sides of the significant letter: a combined token like
  # `-fx` or `-xf` must match regardless of what other letters surround f/d.
  # `git[[:space:]].*clean` (not `+clean`): skip over any global options
  # between "git" and the subcommand (`git -C . clean -f -d`).
  [[ "$command" =~ git[[:space:]].*clean[[:space:]].*(-[A-Za-z]*f[A-Za-z]*|--force)($B|$) ]] && clean_force=1
  [[ "$command" =~ git[[:space:]].*clean[[:space:]].*(-[A-Za-z]*d[A-Za-z]*|--directories|--dirs)($B|$) ]] && clean_dirs=1
  if [[ $clean_force -eq 1 && $clean_dirs -eq 1 ]]; then
    block "git clean --force --dirs (separate flags)"
  fi
fi

exit 0
