#!/usr/bin/env bash
# Shared input inventory for candidate build, audit and repeatability checks.
candidate_complete() (
  set -euo pipefail
  cd "$1"
  local name
  for name in status provenance.txt SHA256SUMS; do
    [[ -f "$name" && ! -L "$name" ]] || { printf 'Candidate requires regular metadata: %s\n' "$name" >&2; exit 1; }
  done
  [[ "$(cat status)" == built ]] || { printf 'Candidate build is incomplete\n' >&2; exit 1; }
)

candidate_inventory() (
  set -euo pipefail
  cd "$1"
  [[ -d source && ! -L source && -d bin && ! -L bin ]] || { printf 'Candidate source/bin must be real directories\n' >&2; exit 1; }
  local special
  special="$(find source bin ! -type f ! -type d -print -quit)"
  [[ -z "$special" ]] || { printf 'Candidate contains a non-regular file: %s\n' "$special" >&2; exit 1; }
  find source -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
  find bin -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
)
