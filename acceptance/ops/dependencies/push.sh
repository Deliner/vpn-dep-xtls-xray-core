#!/usr/bin/env bash
set -euo pipefail

repository="${1:-}"
[[ "$repository" =~ ^Deliner/vpn-dep-[a-z0-9._-]+$ ]] || { printf 'Expected a maintained Deliner dependency fork\n' >&2; exit 2; }
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
target="$root/build/dependencies/checkouts/${repository#*/}"
[[ -d "$target/.git" && ! -L "$target" ]] || { printf 'Expected an owned dependency checkout\n' >&2; exit 2; }
[[ "$(gh api --hostname github.com user --jq .login)" == Deliner ]] || { printf 'Authenticated account changed\n' >&2; exit 2; }
[[ "$(git -C "$target" remote get-url --push origin)" == "https://github.com/$repository.git" ]] || { printf 'Unexpected push destination\n' >&2; exit 2; }
[[ "$(git -C "$target" branch --show-current)" == nhop-maintained ]] || { printf 'Expected maintenance branch\n' >&2; exit 2; }
[[ -z "$(git -C "$target" status --porcelain=v1)" ]] || { printf 'Commit and review checkout changes before publishing\n' >&2; exit 2; }

# Preserved upstream workflows must not start merely because a maintenance branch is pushed.
gh api --hostname github.com --method PUT "repos/$repository/actions/permissions" -F enabled=false

# No force push: a concurrently advanced maintenance branch must be reconciled.
GIT_TERMINAL_PROMPT=0 git -C "$target" -c credential.helper= \
  -c 'credential.helper=!gh auth git-credential' push --porcelain origin HEAD:refs/heads/nhop-maintained
