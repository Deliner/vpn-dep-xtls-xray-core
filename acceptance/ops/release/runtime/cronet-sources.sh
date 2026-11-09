#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && ! -e "$1" && ! -L "$1" ]] || exit 2
root="$PWD"
mkdir "$1"
output="$(cd "$1" && pwd -P)"
work="$(mktemp -d "$root/build/cronet-sources-XXXXXXXX")"
trap 'result=$?; rm -rf "$work"; if (( result != 0 )); then rm -rf "$output"; fi' EXIT
for component in cronet-go naiveproxy; do
  row="$(awk -F '\t' -v component="$component" '$1 == component {print}' ops/runtime/cronet/source.tsv)"
  [[ -n "$row" && "$row" != *$'\n'* ]] || exit 2
  IFS=$'\t' read -r label url revision tree <<< "$row"
  [[ "$url" =~ ^https://github.com/Deliner/vpn-dep-sagernet-(cronet-go|naiveproxy)\.git$ && "$revision" =~ ^[0-9a-f]{40}$ && "$tree" =~ ^[0-9a-f]{40}$ ]] || exit 2
  checkout="$root/build/dependencies/checkouts/${url##*/}"
  checkout="${checkout%.git}"
  [[ "$(git -C "$checkout" remote get-url origin)" == "$url" ]] || exit 2
  [[ "$(git -C "$checkout" rev-parse HEAD)" == "$revision" && "$(git -C "$checkout" rev-parse HEAD^{tree})" == "$tree" ]] || exit 2
  [[ -z "$(git -C "$checkout" status --porcelain --untracked-files=all)" ]] || exit 2
  expected="$(awk -F '\t' -v component="$component" '$1 == component {print $2}' ops/release/runtime/cronet-sources.tsv)"
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || exit 2
  git -C "$checkout" archive --format=tar "$revision" > "$work/$component.tar"
  go run ./cmd/runtimepack source-package "$expected" "$work/$component.tar" "$output/$component"
  printf '%s\n' "$row" >> "$output/source.tsv"
done
(cd "$output"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/evidence.sha256"
