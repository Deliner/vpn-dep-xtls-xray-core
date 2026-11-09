#!/usr/bin/env bash
# Shared identity check for source-only tests against the maintained NaiveProxy pin.
cronet_verify_source() {
  local row label
  row="$(awk -F '\t' '$1 == "naiveproxy" {print}' "$root/ops/runtime/cronet/source.tsv")" || return 2
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
