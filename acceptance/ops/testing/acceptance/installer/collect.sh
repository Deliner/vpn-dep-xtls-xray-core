#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
collection="$1" commit="$2" snapshot="$3"
for arch in amd64 arm64; do
  just acceptance-installer-verify "$collection/installer-linux-$arch" "$arch" "$commit" "$snapshot" \
    > "$collection/installer-$arch-verification.log" 2>&1
done
