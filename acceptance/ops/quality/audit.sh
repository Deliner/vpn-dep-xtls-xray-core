#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)); then
  printf 'usage: audit.sh SCANNER REPORT_DIRECTORY\n' >&2
  exit 2
fi
scanner="$1"
report_dir="$2"
mkdir -p "$report_dir"
for target in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
  printf 'Scan not completed in this run.\n' > "$report_dir/$target.txt"
done
date -u '+started_at=%Y-%m-%dT%H:%M:%SZ' > "$report_dir/scanner.txt"
if "$scanner" -version >> "$report_dir/scanner.txt" 2>&1; then
  cat "$report_dir/scanner.txt"
else
  status=$?
  cat "$report_dir/scanner.txt" >&2
  exit "$status"
fi

status=0
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  report="$report_dir/$os-$arch.txt"
  printf '\nScanning %s (including tests)\n' "$target"
  date -u '+started_at=%Y-%m-%dT%H:%M:%SZ' > "$report"
  # Text mode returns nonzero for findings. JSON/SARIF output alone is not a gate.
  if GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 "$scanner" -test -show traces,verbose ./... >> "$report" 2>&1; then
    result=0
  else
    result=$?
    status=1
  fi
  printf '\n%s: scanner exit status %s\n' "$target" "$result" >> "$report"
  cat "$report"
done
exit "$status"
