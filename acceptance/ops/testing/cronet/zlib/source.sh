#!/usr/bin/env bash
# Immutable source selection; never patch the maintained dependency checkout.
source "$root/ops/testing/cronet/source.sh"

window_prepare_source() {
  cronet_verify_source
  cp "$root/ops/runtime/cronet/source.tsv" "$work/source.tsv"
  cp "$recipe/"* "$work/inputs/"
  cp "$root/ops/testing/cronet/source.sh" "$work/inputs/checkout-source.sh"
  git -C "$checkout" archive "$revision" src/third_party/zlib > "$work/inputs/zlib-source.tar"
  tar -xf "$work/inputs/zlib-source.tar" --strip-components=3 -C "$work/baseline"
  [[ -z "$(find "$work/baseline" -type l -print)" ]] || return 2
  cp -R "$work/baseline/." "$work/patched/"
  local patch_state=private-patch
  if git -C "$root" apply --reverse --check --directory="$relative/baseline" "$work/inputs/window.patch" 2>/dev/null; then
    git -C "$root" apply --reverse --directory="$relative/baseline" "$work/inputs/window.patch"
    patch_state=current-source-fixed
  else
    git -C "$root" apply --check --directory="$relative/patched" "$work/inputs/window.patch"
    git -C "$root" apply --directory="$relative/patched" "$work/inputs/window.patch"
  fi
  {
    printf 'url=%s\nrevision=%s\ntree=%s\n' "$url" "$revision" "$tree"
    printf 'upstream_fix=2fd31f00740ea183b640f3f5c12659420e189c76\n'
    printf 'patch_state=%s\n' "$patch_state"
    printf 'host_source_test_only=true\n'
    uname -sm
    clang --version
    clang -dumpmachine
  } > "$work/provenance.txt"
}
