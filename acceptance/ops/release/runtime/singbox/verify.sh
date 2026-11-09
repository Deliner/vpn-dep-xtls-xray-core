#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && ! -L "$1" ]] || exit 2
root="$PWD"
stage="$(cd "$1" && pwd -P)"
names=(sing-box-1.14.4-linux-amd64.tar.gz sing-box-1.14.4-linux-arm64.tar.gz policy.json cronet-go-source.tar.gz cronet-go-NOTICE.txt naiveproxy-source.tar.gz naiveproxy-NOTICE.txt cronet-source.tsv cronet-publication.tsv publication.tsv notes.md)
count="$(find "$stage" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')"
[[ "$count" == 12 ]] || { printf 'Expected exactly twelve sing-box release assets.\n' >&2; exit 2; }
for name in "${names[@]}" SHA256SUMS; do
  [[ -f "$stage/$name" && ! -L "$stage/$name" ]] || exit 2
done
cmp ops/release/runtime/sing-box.json "$stage/policy.json"
cmp ops/release/runtime/singbox/publication.tsv "$stage/publication.tsv"
cmp ops/release/runtime/singbox/notes.md "$stage/notes.md"
cmp ops/runtime/cronet/source.tsv "$stage/cronet-source.tsv"
cmp ops/release/runtime/cronet/publication.tsv "$stage/cronet-publication.tsv"
for arch in amd64 arm64; do
  go run ./cmd/runtimepack singbox-archive-verify ops/release/runtime/sing-box.json "$arch" "$stage/sing-box-1.14.4-linux-$arch.tar.gz"
done
for component in cronet-go naiveproxy; do
  expected="$(awk -F '\t' -v component="$component" '$1 == component {print $2}' ops/release/runtime/cronet-sources.tsv)"
  published="$(awk -F '\t' -v component="$component" '$1 == component {print $2}' "$stage/cronet-publication.tsv")"
  [[ "$expected" == "$published" && "$expected" =~ ^[0-9a-f]{64}$ ]] || exit 2
  go run ./cmd/runtimepack source-verify "$expected" "$stage/$component-source.tar.gz" "$stage/$component-NOTICE.txt"
done
work="$(mktemp -d "$root/build/singbox-verify-XXXXXXXX")"
trap 'rm -rf "$work"' EXIT
(cd "$stage"; shasum -a 256 "${names[@]}") > "$work/SHA256SUMS"
cmp "$work/SHA256SUMS" "$stage/SHA256SUMS"
printf 'sing-box release runtimes, complete sources and notices verified.\n'
