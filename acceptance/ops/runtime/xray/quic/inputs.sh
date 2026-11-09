#!/usr/bin/env bash
# Source-only verification shared by native tests and experimental Xray builds.
quic_regression_verify() (
  set -euo pipefail
  local root="$1" source_run="$2" inventory="$3" recipe path name
  recipe="$root/ops/runtime/xray/quic"
  case "$source_run" in "$root/build/quic-mtu-regression/"*) ;; *) exit 2 ;; esac
  name="${source_run#"$root/build/quic-mtu-regression/"}"
  case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
  [[ ! -L "$root/build" && ! -L "$root/build/quic-mtu-regression" ]]
  [[ -d "$source_run" && ! -L "$source_run" && "$(cat "$source_run/status")" == passed ]]
  [[ -z "$(find "$source_run" -type l -print)" ]]
  (cd "$source_run"; find . -type d -name .git -prune -o -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$inventory"
  cmp "$source_run/evidence.sha256" "$inventory"
  for path in "$recipe/"*.patch "$recipe/regression.sh" "$recipe/../go.mod" "$recipe/../go.sum"; do
    grep -Fxq "$(shasum -a 256 "$path")" "$source_run/provenance.txt"
  done
)
