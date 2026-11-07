#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$root/ops/runtime/integrity.sh"
[[ $# == 1 ]] || exit 2
candidate="$(cd "$1" && pwd)"
reports="$candidate/audit"
[[ ! -L "$reports" && ( ! -e "$reports" || -d "$reports" ) ]] || { printf 'Audit output must be a real directory\n' >&2; exit 1; }
mkdir -p "$reports"
for name in source.txt binary.txt actual.sha256 scanner.txt provenance.txt; do
  [[ ! -L "$reports/$name" && ( ! -e "$reports/$name" || -f "$reports/$name" ) ]] || { printf 'Audit output must be a regular file: %s\n' "$name" >&2; exit 1; }
done
for scan in source binary; do printf 'Scan not completed in this run.\n' > "$reports/$scan.txt"; done
candidate_complete "$candidate"
engine="$(awk -F = '$1 == "engine" { print $2 }' "$candidate/provenance.txt")"
target="$(awk -F = '$1 == "target" { print $2 }' "$candidate/provenance.txt")"
case "$engine" in xray) package=./main ;; sing-box) package=./cmd/sing-box ;; *) exit 2 ;; esac
tags="$(sed -n 's/^build_tags=//p' "$candidate/provenance.txt")"
cgo="$(sed -n 's/^build_cgo=//p' "$candidate/provenance.txt")"
cgo="${cgo:-0}"
scanner="$root/build/tools/bin/govulncheck"
candidate_inventory "$candidate" > "$reports/actual.sha256"
cmp "$candidate/SHA256SUMS" "$reports/actual.sha256" || { printf 'Candidate changed after build\n' >&2; exit 1; }
date -u '+started_at=%Y-%m-%dT%H:%M:%SZ' > "$reports/scanner.txt"
"$scanner" -version >> "$reports/scanner.txt" 2>&1
cp "$candidate/provenance.txt" "$reports/provenance.txt"

status=0
for scan in source binary; do
  if [[ "$scan" == source ]]; then
    arguments=(-C "$candidate/source" -tags "$tags" -show traces,verbose "$package")
  else
    arguments=(-mode binary "$candidate/bin/$engine")
  fi
  if GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED="$cgo" GOWORK=off \
    "$scanner" "${arguments[@]}" > "$reports/$scan.txt" 2>&1; then
    result=0
  else
    result=$?
    status=1
  fi
  printf '\nscanner_exit_status=%s\n' "$result" >> "$reports/$scan.txt"
  cat "$reports/$scan.txt"
done
exit "$status"
