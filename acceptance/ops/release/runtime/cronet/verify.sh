#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 ]] || exit 2
root="$PWD"
stage="$(cd "$1" && pwd -P)"
names=(cronet-linux-amd64.zip cronet-linux-arm64.zip policy.json cronet-go-source.tar.gz cronet-go-NOTICE.txt naiveproxy-source.tar.gz naiveproxy-NOTICE.txt source.tsv publication.tsv notes.md)
[[ ! -L "$1" ]] || exit 2
count="$(find "$stage" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')"
[[ "$count" == 11 ]] || { printf 'Expected exactly eleven Cronet release assets.\n' >&2; exit 2; }
for name in "${names[@]}" SHA256SUMS; do
  [[ -f "$stage/$name" && ! -L "$stage/$name" ]] || exit 2
done
cmp ops/release/runtime/cronet/publication.tsv "$stage/publication.tsv"
cmp ops/release/runtime/cronet/notes.md "$stage/notes.md"
cmp ops/runtime/cronet/source.tsv "$stage/source.tsv"
work="$(mktemp -d "$root/build/cronet-verify-XXXXXXXX")"
trap 'rm -rf "$work"' EXIT
mkdir "$work/binary"
for name in cronet-linux-amd64.zip cronet-linux-arm64.zip policy.json; do
  cp "$stage/$name" "$work/binary/$name"
done
(cd "$work/binary"; shasum -a 256 cronet-linux-amd64.zip cronet-linux-arm64.zip policy.json) > "$work/binary/SHA256SUMS"
go run ./cmd/runtimepack cronet-package-verify ops/release/runtime/cronet.json "$work/binary"
for component in cronet-go naiveproxy; do
  expected="$(awk -F '\t' -v component="$component" '$1 == component {print $2}' ops/release/runtime/cronet-sources.tsv)"
  published="$(awk -F '\t' -v component="$component" '$1 == component {print $2}' "$stage/publication.tsv")"
  [[ "$expected" == "$published" && "$expected" =~ ^[0-9a-f]{64}$ ]] || exit 2
  go run ./cmd/runtimepack source-verify "$expected" "$stage/$component-source.tar.gz" "$stage/$component-NOTICE.txt"
done
(cd "$stage"; shasum -a 256 "${names[@]}") > "$work/SHA256SUMS"
cmp "$work/SHA256SUMS" "$stage/SHA256SUMS"
printf 'Cronet release libraries, complete sources and notices verified.\n'
