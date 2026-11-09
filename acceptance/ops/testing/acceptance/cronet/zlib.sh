#!/usr/bin/env bash
set -euo pipefail
[[ $# == 1 && "$(uname -s)" == Linux ]] || exit 2
report="$1"
row="$(awk -F '\t' '$1 == "naiveproxy" {print}' ops/runtime/cronet/source.tsv)"
[[ -n "$row" && "$row" != *$'\n'* ]] || exit 2
IFS=$'\t' read -r component url revision tree <<< "$row"
repository="${url#https://github.com/}"
just dependency-checkout "${repository%.git}" "$revision" > "$report/zlib-source-fetch.log" 2>&1
just cronet-zlib-window-test build/reports/cronet-zlib/native > "$report/zlib-driver.log" 2>&1
cp -R build/reports/cronet-zlib/native "$report/zlib-window"
