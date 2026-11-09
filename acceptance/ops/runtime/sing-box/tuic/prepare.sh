#!/usr/bin/env bash
# Keep a patched, attributed dependency inside the candidate's source inventory.
candidate_tuic_prepare() {
  local module=github.com/sagernet/sing-quic
  local version=v0.7.2-0.20261020082049-7e0a5869a5f4
  local sum='h1:QuhAb8QW0pKYmSvfj/nK91libunBwVJiWb852yidlxI='
  local directory destination="$work/source/nhop-dependencies/sing-quic"
  [[ "$(go -C "$work/source" list -m -f '{{.Version}}' "$module")" == "$version" ]] || return 2
  [[ "$(go -C "$work/source" list -m -f '{{if .Replace}}replaced{{end}}' "$module")" == '' ]] || return 2
  go -C "$work/source" mod download "$module"
  go -C "$work/source" mod verify
  [[ "$(go -C "$work/source" list -m -f '{{.Sum}}' "$module")" == "$sum" ]] || return 2
  directory="$(go -C "$work/source" list -m -f '{{.Dir}}' "$module")"
  [[ "$directory" == "$GOMODCACHE/$module@$version" && -d "$directory" && ! -L "$directory" ]] || return 2
  [[ -z "$(find "$directory" ! -type f ! -type d -print -quit)" ]] || return 2
  [[ ! -e "$work/source/nhop-dependencies" && ! -L "$work/source/nhop-dependencies" ]] || return 2
  mkdir "$work/source/nhop-dependencies"
  cp -R "$directory" "$destination"
  chmod -R u+w "$destination"
  candidate_tuic_patch "$destination"
  go -C "$work/source" mod edit "-replace=$module@$version=./nhop-dependencies/sing-quic"
  [[ "$(go -C "$work/source" list -m -f '{{.Replace.Dir}}' "$module")" == "$destination" ]] || return 2
  {
    printf 'tuic_module=%s\ntuic_version=%s\ntuic_upstream_sum=%s\n' "$module" "$version" "$sum"
    printf 'tuic_source=source/nhop-dependencies/sing-quic\ntuic_patch_state=applied\n'
    (cd "$root"; shasum -a 256 ops/runtime/sing-box/tuic/prepare.sh ops/runtime/sing-box/tuic/{server-listener,service-shutdown}.patch)
  } > "$work/patch.txt"
  cp "$work/patch.txt" "$work/source/nhop-dependencies/origin.txt"
}

candidate_tuic_patch() {
  local destination="$1" name digest
  for name in server-listener service-shutdown; do
    case "$name" in
      server-listener) digest=e3d0e8947ed6f9daef030654db558a5b5ee7fc86aa7e0c9ee5e532a2f1464105 ;;
      service-shutdown) digest=5d36cf1cce9ae778354b6118efd5103d18be64c20175cff00600d22d4de48b28 ;;
    esac
    [[ -f "$recipe/tuic/$name.patch" && ! -L "$recipe/tuic/$name.patch" ]] || return 2
    printf '%s  %s\n' "$digest" "$recipe/tuic/$name.patch" | shasum -a 256 --check
  done
  # A private temporary Git directory prevents discovery of the enclosing project.
  git -C "$destination" init --quiet
  for name in server-listener service-shutdown; do
    git -C "$destination" apply --check --whitespace=error-all "$recipe/tuic/$name.patch"
    git -C "$destination" apply --whitespace=error-all "$recipe/tuic/$name.patch"
  done
  rm -rf "$destination/.git"
}
