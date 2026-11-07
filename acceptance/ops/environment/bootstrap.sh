#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
platform_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  arm64|aarch64) platform_arch=arm64 ;;
  x86_64) platform_arch=amd64 ;;
  *) printf 'Unsupported CPU architecture\n' >&2; exit 1 ;;
esac
toolchain_root="$project_root/build/toolchains"
mkdir -p "$toolchain_root" "$project_root/build/tools/bin"
# Keep generated tools and fixtures outside the application's Go module traversal.
printf 'module vpn-script-build-artifacts\n\ngo 1.27.3\n' > "$project_root/build/go.mod"
stage="$(mktemp -d "$toolchain_root/.bootstrap-XXXXXXXX")"
trap 'rm -rf "$stage"' EXIT

download() {
  local tool="$1" row url digest
  row="$(awk -F '\t' -v tool="$tool" -v platform="$platform_os/$platform_arch" \
    '$1 == tool && $2 == platform { print $3 " " $4 }' "$project_root/ops/environment/toolchains.tsv")"
  read -r url digest <<< "$row"
  [[ -n "$url" && "$digest" =~ ^[a-f0-9]{64}$ ]] || { printf 'Missing pinned toolchain: %s\n' "$tool" >&2; exit 1; }
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 \
    --proto '=https' --proto-redir '=https' --output "$stage/$tool.tar.gz" "$url"
  printf '%s  %s\n' "$digest" "$stage/$tool.tar.gz" | shasum -a 256 --check
}

install_go() {
  local expected actual
  expected="$(awk '$1 == "go" { print $2 }' "$project_root/go.mod")"
  actual="$("$toolchain_root/go/bin/go" version 2>/dev/null || true)"
  [[ "$actual" == "go version go$expected "* ]] && return
  download go
  mkdir "$stage/go-unpack"
  tar -xzf "$stage/go.tar.gz" -C "$stage/go-unpack"
  actual="$("$stage/go-unpack/go/bin/go" version)"
  [[ "$actual" == "go version go$expected "* ]] || { printf 'go.mod and toolchain pin differ\n' >&2; exit 1; }
  if [[ -e "$toolchain_root/go" ]]; then
    printf 'Existing project Go differs; move build/toolchains/go aside and retry.\n' >&2
    exit 1
  fi
  mv "$stage/go-unpack/go" "$toolchain_root/go"
}

install_just() {
  local target="$project_root/build/tools/bin/just"
  [[ "$("$target" --version 2>/dev/null || true)" == 'just 1.58.0' ]] && return
  download just
  mkdir "$stage/just-unpack"
  tar -xzf "$stage/just.tar.gz" -C "$stage/just-unpack" just
  [[ "$("$stage/just-unpack/just" --version)" == 'just 1.58.0' ]] || exit 1
  install -m 0755 "$stage/just-unpack/just" "$target"
}

install_go
install_just
printf 'Project toolchains ready. Run: build/tools/bin/just doctor\n'
