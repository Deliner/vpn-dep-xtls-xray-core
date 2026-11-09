#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 4 && ! -e "$4" && ! -L "$4" ]] || exit 2
amd64="$(cd "$1" && pwd -P)"
arm64="$(cd "$2" && pwd -P)"
cronet="$(cd "$3" && pwd -P)"
bash ops/release/runtime/cronet/verify.sh "$cronet"
mkdir "$4"
output="$(cd "$4" && pwd -P)"
trap 'result=$?; if (( result != 0 )); then rm -rf "$output"; fi' EXIT
cp "$amd64/sing-box-1.14.4-linux-amd64.tar.gz" "$arm64/sing-box-1.14.4-linux-arm64.tar.gz" "$output/"
cp ops/release/runtime/sing-box.json "$output/policy.json"
for component in cronet-go naiveproxy; do
  cp "$cronet/$component-source.tar.gz" "$cronet/$component-NOTICE.txt" "$output/"
done
cp "$cronet/source.tsv" "$output/cronet-source.tsv"
cp "$cronet/publication.tsv" "$output/cronet-publication.tsv"
cp ops/release/runtime/singbox/publication.tsv ops/release/runtime/singbox/notes.md "$output/"
(cd "$output"; shasum -a 256 sing-box-1.14.4-linux-amd64.tar.gz sing-box-1.14.4-linux-arm64.tar.gz policy.json cronet-go-source.tar.gz cronet-go-NOTICE.txt naiveproxy-source.tar.gz naiveproxy-NOTICE.txt cronet-source.tsv cronet-publication.tsv publication.tsv notes.md) > "$output/SHA256SUMS"
bash ops/release/runtime/singbox/verify.sh "$output"
