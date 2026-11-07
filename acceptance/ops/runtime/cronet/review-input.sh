#!/usr/bin/env bash
set -euo pipefail
umask 077

component="${1:-}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
case "$component" in clang-mac-arm64|gn-mac-arm64|sysroot-linux-arm64|sysroot-linux-amd64|ninja-mac|pgo-linux|pgo-mac-arm64) ;; *) exit 2 ;; esac
row="$(awk -F '\t' -v component="$component" '$1 == component {print}' "$root/ops/runtime/cronet/inputs.tsv")"
[[ -n "$row" && "$row" != *$'\n'* ]] || { printf 'Expected one reviewed source URL\n' >&2; exit 2; }
IFS=$'\t' read -r label expected url <<< "$row"
[[ "$url" == https://* && ( "$expected" == - || "$expected" =~ ^[0-9a-f]{64}$ ) ]] || exit 2
parent="$root/build/native-inputs/$component"
for directory in "$root/build" "$root/build/native-inputs" "$parent"; do
  [[ ! -L "$directory" ]] || { printf 'Symlink input parent refused\n' >&2; exit 2; }
  mkdir -p "$directory"
done
stage="$(mktemp -d "$parent/review-XXXXXXXX")"
printf '%s\n' "$url" > "$stage/url.txt"
# This command only obtains review evidence. It never extracts or executes input.
encoding=()
if [[ "$component" == pgo-* ]]; then encoding=(--header 'Accept-Encoding: gzip'); fi
curl --fail --location --retry 2 --connect-timeout 20 --max-time 1200 \
  --proto '=https' --proto-redir '=https' --dump-header "$stage/headers.txt" \
  "${encoding[@]}" --output "$stage/artifact" "$url"
if [[ "$component" == pgo-* ]]; then
  # GCS normally transcodes these objects. Verify its stored gzip bytes before
  # comparing the decoded profile with our SHA pin; neither representation runs.
  stored_md5="$(awk 'tolower($0) ~ /^x-goog-hash:/ && /md5=/ {sub(/.*md5=/, ""); sub(/[, \r].*/, ""); print}' "$stage/headers.txt" | tail -1)"
  actual_md5="$(openssl dgst -md5 -binary "$stage/artifact" | openssl base64 -A)"
  [[ -n "$stored_md5" && "$actual_md5" == "$stored_md5" ]] || { printf 'Stored PGO transport digest differs\n' >&2; exit 1; }
  mv "$stage/artifact" "$stage/stored.gz"
  gzip -dc "$stage/stored.gz" > "$stage/artifact"
fi
actual="$(shasum -a 256 "$stage/artifact" | awk '{print $1}')"
size="$(wc -c < "$stage/artifact" | tr -d ' ')"
printf '%s\t%s\t%s\t%s\n' "$component" "$actual" "$size" "$url" > "$stage/observed.tsv"
if [[ "$expected" != - && "$actual" != "$expected" ]]; then
  printf 'Native input differs from its reviewed checksum; preserved at %s\n' "$stage" >&2
  exit 1
fi
cat "$stage/observed.tsv"
printf 'Review only; no artifact executed: %s\n' "$stage"
