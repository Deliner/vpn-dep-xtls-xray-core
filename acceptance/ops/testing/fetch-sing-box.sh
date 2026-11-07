#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
platform_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  arm64|aarch64) platform_arch=arm64 ;;
  x86_64) platform_arch=amd64 ;;
  *) printf 'Unsupported CPU architecture\n' >&2; exit 1 ;;
esac
row="$(awk -F '\t' -v platform="$platform_os/$platform_arch" '$1 == platform { print $2 " " $3 }' "$project_root/ops/testing/sing-box.tsv")"
read -r url digest <<< "$row"
[[ -n "$url" && "$digest" =~ ^[a-f0-9]{64}$ ]] || exit 1
destination="$project_root/build/sing-box-test"
mkdir -p "$destination"
stage="$(mktemp -d "$destination/.fetch-XXXXXXXX")"
trap 'rm -rf "$stage"' EXIT
curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 \
  --proto '=https' --proto-redir '=https' --output "$stage/runtime.tar.gz" "$url"
printf '%s  %s\n' "$digest" "$stage/runtime.tar.gz" | shasum -a 256 --check
member="sing-box-1.14.4-$platform_os-$platform_arch/sing-box"
tar -xzf "$stage/runtime.tar.gz" -C "$stage" "$member"
install -m 0755 "$stage/$member" "$destination/sing-box"
if [[ "$platform_os" == linux ]]; then
  library="sing-box-1.14.4-$platform_os-$platform_arch/libcronet.so"
  tar -xzf "$stage/runtime.tar.gz" -C "$stage" "$library"
  install -m 0755 "$stage/$library" "$destination/libcronet.so"
fi
"$destination/sing-box" version
