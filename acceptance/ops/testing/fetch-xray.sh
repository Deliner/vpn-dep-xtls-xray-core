#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
platform_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  arm64|aarch64) platform_arch=arm64 ;;
  x86_64) platform_arch=amd64 ;;
  *) printf 'Unsupported CPU architecture\n' >&2; exit 1 ;;
esac
row="$(awk -F '\t' -v platform="$platform_os/$platform_arch" \
  '$1 == platform { print $2 " " $3 }' "$project_root/ops/testing/xray.tsv")"
read -r url digest <<< "$row"
[[ -n "$url" && "$digest" =~ ^[a-f0-9]{64}$ ]] || exit 1
destination="$project_root/build/xray-test"
mkdir -p "$destination"
stage="$(mktemp -d "$destination/.fetch-XXXXXXXX")"
trap 'rm -rf "$stage"' EXIT
curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 \
  --proto '=https' --proto-redir '=https' --output "$stage/xray.zip" "$url"
printf '%s  %s\n' "$digest" "$stage/xray.zip" | shasum -a 256 --check
unzip -j "$stage/xray.zip" xray -d "$stage"
install -m 0755 "$stage/xray" "$destination/xray"
"$destination/xray" version
