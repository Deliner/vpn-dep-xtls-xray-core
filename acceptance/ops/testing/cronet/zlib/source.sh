#!/usr/bin/env bash
# Immutable source selection; never patch the maintained dependency checkout.
window_verify_checkout() {
  local row label
  row="$(awk -F '\t' '$1 == "naiveproxy" {print}' "$root/ops/runtime/cronet/source.tsv")"
  [[ -n "$row" && "$row" != *$'\n'* ]] || return 2
  IFS=$'\t' read -r label url revision tree <<< "$row"
  [[ "$url" == https://github.com/Deliner/vpn-dep-sagernet-naiveproxy.git ]] || return 2
  [[ "$revision" =~ ^[0-9a-f]{40}$ && "$tree" =~ ^[0-9a-f]{40}$ ]] || return 2
  checkout="$root/build/dependencies/checkouts/vpn-dep-sagernet-naiveproxy"
  [[ "$(git -C "$checkout" remote get-url origin)" == "$url" ]] || return 2
  [[ "$(git -C "$checkout" rev-parse HEAD)" == "$revision" ]] || return 2
  [[ "$(git -C "$checkout" rev-parse HEAD^{tree})" == "$tree" ]] || return 2
  [[ -z "$(git -C "$checkout" status --porcelain --untracked-files=all)" ]] || return 2
}

window_prepare_source() {
  window_verify_checkout
  cp "$root/ops/runtime/cronet/source.tsv" "$work/source.tsv"
  cp "$recipe/"* "$work/inputs/"
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
