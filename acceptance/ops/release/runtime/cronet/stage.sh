#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 && ! -e "$3" && ! -L "$3" ]] || exit 2
binary="$(cd "$1" && pwd -P)"
sources="$(cd "$2" && pwd -P)"
go run ./cmd/runtimepack cronet-package-verify ops/release/runtime/cronet.json "$binary"
mkdir "$3"
output="$(cd "$3" && pwd -P)"
trap 'result=$?; if (( result != 0 )); then rm -rf "$output"; fi' EXIT
for name in cronet-linux-amd64.zip cronet-linux-arm64.zip policy.json; do
  cp "$binary/$name" "$output/$name"
done
for component in cronet-go naiveproxy; do
  for name in source.tar.gz NOTICE.txt; do
    [[ -f "$sources/$component/$name" && ! -L "$sources/$component/$name" ]] || exit 2
    cp "$sources/$component/$name" "$output/$component-$name"
  done
done
cp ops/runtime/cronet/source.tsv "$output/source.tsv"
cp ops/release/runtime/cronet/publication.tsv ops/release/runtime/cronet/notes.md "$output/"
(cd "$output"; shasum -a 256 cronet-linux-amd64.zip cronet-linux-arm64.zip policy.json cronet-go-source.tar.gz cronet-go-NOTICE.txt naiveproxy-source.tar.gz naiveproxy-NOTICE.txt source.tsv publication.tsv notes.md) > "$output/SHA256SUMS"
bash ops/release/runtime/cronet/verify.sh "$output"
