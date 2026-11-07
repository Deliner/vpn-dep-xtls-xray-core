#!/usr/bin/env bash
set -euo pipefail
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/integrity.sh"
for candidate in "$1" "$2"; do
  candidate_complete "$candidate"
  # A temporary file preserves producer failures; process substitution would not.
  actual="$(mktemp "$candidate/.compare-XXXXXXXX")"
  trap 'rm -f "$actual"' EXIT
  candidate_inventory "$candidate" > "$actual"
  cmp "$candidate/SHA256SUMS" "$actual"
  rm "$actual"
done
cmp "$1/SHA256SUMS" "$2/SHA256SUMS"
printf 'Identical source inventories and runtime bytes.\n'
shasum -a 256 "$1"/bin/* "$2"/bin/*
