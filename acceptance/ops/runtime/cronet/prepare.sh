#!/usr/bin/env bash
# Source preparation and reviewed input selection for native Cronet builds.
native_archive_source() {
  local component="$1" destination="$2" row label url revision tree checkout
  row="$(awk -F '\t' -v component="$component" '$1 == component {print}' "$recipe/source.tsv")"
  [[ -n "$row" && "$row" != *$'\n'* ]] || return 2
  IFS=$'\t' read -r label url revision tree <<< "$row"
  [[ "$url" =~ ^https://github.com/Deliner/vpn-dep-[a-z0-9._-]+\.git$ && "$revision" =~ ^[0-9a-f]{40}$ && "$tree" =~ ^[0-9a-f]{40}$ ]] || return 2
  checkout="$root/build/dependencies/checkouts/${url##*/}"
  checkout="${checkout%.git}"
  [[ "$(git -C "$checkout" remote get-url origin)" == "$url" ]] || return 2
  [[ "$(git -C "$checkout" rev-parse HEAD)" == "$revision" ]] || return 2
  [[ "$(git -C "$checkout" rev-parse HEAD^{tree})" == "$tree" ]] || return 2
  [[ -z "$(git -C "$checkout" status --porcelain --untracked-files=all)" ]] || return 2
  git -C "$checkout" archive --format=tar "$revision" > "$work/inputs/$component-source.tar"
  mkdir -p "$destination"
  tar -xf "$work/inputs/$component-source.tar" -C "$destination"
  printf '%s\n' "$row" >> "$work/source.tsv"
}

native_pin_input() {
  local component="$1" row label digest url candidate target
  row="$(awk -F '\t' -v component="$component" '$1 == component {print}' "$recipe/inputs.tsv")"
  [[ -n "$row" && "$row" != *$'\n'* ]] || return 2
  IFS=$'\t' read -r label digest url <<< "$row"
  [[ "$digest" =~ ^[0-9a-f]{64}$ && "$url" == https://* ]] || return 2
  target="$work/inputs/$component"
  for candidate in "$root/build/native-inputs/$component"/review-*/artifact; do
    [[ -f "$candidate" && ! -L "$candidate" ]] || continue
    [[ "$(shasum -a 256 "$candidate" | awk '{print $1}')" == "$digest" ]] || continue
    cp "$candidate" "$target"
    printf '%s  %s\n' "$digest" "$target" | shasum -a 256 --check >/dev/null
    printf '%s\n' "$row" >> "$work/inputs.tsv"
    printf '%s\n' "$target"
    return 0
  done
  printf 'Missing pinned native input; run just cronet-review-input %s\n' "$component" >&2
  return 1
}
