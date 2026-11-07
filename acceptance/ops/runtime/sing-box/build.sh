#!/usr/bin/env bash
source "$recipe/cronet.sh"

candidate_configure() {
  candidate_cronet_prepare
  build_label=nhop-fork-cronet154-1
  build_package=./cmd/sing-box
  build_tags="$(cat "$work/source/release/DEFAULT_BUILD_TAGS"),nhop_fork_cronet154_1"
  build_flags="-X github.com/sagernet/sing-box/constant.Version=$version $(cat "$work/source/release/LDFLAGS") -s -w -buildid="
  build_cgo=1
  if [[ "$target_os" == linux ]]; then
    build_tags+=,with_purego
    build_cgo=0
  elif [[ "$(go env GOHOSTOS)" != darwin ]]; then
    printf 'Darwin sing-box candidates require a macOS C toolchain\n' >&2
    return 2
  fi
}

candidate_companions() {
  [[ "$target_os" == linux ]] || return 0
  cp "$cronet_library" "$work/bin/libcronet.so"
  printf '%s  %s\n' "$cronet_digest" "$work/bin/libcronet.so" | shasum -a 256 --check
  chmod 0755 "$work/bin/libcronet.so"
}
