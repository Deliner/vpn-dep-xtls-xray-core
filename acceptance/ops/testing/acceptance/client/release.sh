#!/usr/bin/env bash
# Bind tested executable bytes to the reviewed publication policy, not self-reported hashes.
client_verify_release_runtime() (
  set -euo pipefail
  [[ $# == 5 ]] || exit 2
  local policies="$1" arch="$2" index engine field digest policy binary
  local binaries=("$3" "$4" "$5")
  case "$arch" in amd64|arm64) ;; *) exit 2 ;; esac
  [[ -d "$policies" && ! -L "$policies" ]] || exit 2
  for index in 0 1 2; do
    engine=sing-box
    field=binary_sha256
    if [[ "$index" == 0 ]]; then engine=xray; fi
    if [[ "$index" == 2 ]]; then field=library_sha256; fi
    policy="$policies/$engine.json"
    binary="${binaries[$index]}"
    [[ -f "$policy" && ! -L "$policy" && -s "$policy" ]] || exit 2
    [[ -f "$binary" && ! -L "$binary" && -s "$binary" ]] || exit 2
    digest="$(jq -er --arg arch "$arch" --arg field "$field" '.targets[$arch][$field] | strings' "$policy")" || exit 2
    [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || exit 2
    printf '%s  %s\n' "$digest" "$binary" | shasum -a 256 --check || exit 1
  done
)
