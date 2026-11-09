#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
for arch in amd64 arm64; do
  report="$1/masque-linux-$arch"
  [[ "$(cat "$report/status")" == passed ]] || exit 1
  just acceptance-masque-verify "$report" "$arch" "$2" "$3" > "$1/masque-$arch-verification.log" 2>&1
done
