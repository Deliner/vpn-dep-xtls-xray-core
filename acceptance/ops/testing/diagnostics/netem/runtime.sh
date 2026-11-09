#!/usr/bin/env bash
# Source-only preparation for the existing lab-masque-netem recipe.
netem_prepare_runtime() (
  set -euo pipefail
  local project="$1" runtime="$2" arch="$3" output="$4" kind name
  source "$project/ops/runtime/integrity.sh"
  runtime="$(cd "$runtime" && pwd -P)"
  for name in status provenance.txt SHA256SUMS; do
    [[ -f "$runtime/$name" && ! -L "$runtime/$name" ]] || return 2
  done
  case "$runtime/" in
    "$project/build/runtime-probes/"*)
      kind=diagnostic-probe
      [[ "$(cat "$runtime/status")" == diagnostic-built ]] || return 2
      (cd "$runtime"; shasum -a 256 --check SHA256SUMS)
      cp "$runtime/SHA256SUMS" "$output/runtime.inventory.sha256"
      ;;
    "$project/build/runtime-candidates/"*)
      kind=runtime-candidate
      candidate_complete "$runtime"
      [[ "$(sed -n 's/^engine=//p' "$runtime/provenance.txt")" == xray ]] || return 2
      candidate_inventory "$runtime" > "$output/runtime.inventory.sha256"
      cmp "$runtime/SHA256SUMS" "$output/runtime.inventory.sha256"
      ;;
    *) printf 'Use a verified runtime candidate or diagnostic probe under build/.\n' >&2; return 2 ;;
  esac
  [[ "$(sed -n 's/^target=//p' "$runtime/provenance.txt" | LC_ALL=C sort -u)" == "linux/$arch" ]] || return 2
  [[ -f "$runtime/bin/xray" && ! -L "$runtime/bin/xray" ]] || return 2
  mkdir "$output/bin"
  cp "$runtime/bin/xray" "$output/bin/xray"
  grep -E '^[a-f0-9]{64}  bin/xray$' "$output/runtime.inventory.sha256" > "$output/runtime.binary.sha256"
  (cd "$output"; shasum -a 256 --check runtime.binary.sha256)
  cp "$runtime/provenance.txt" "$output/provenance.txt"
  printf '%s\n' "$kind" > "$output/runtime-kind"
  if [[ "$kind" == diagnostic-probe ]]; then netem_copy_probe_evidence "$runtime" "$output"; fi
)

netem_copy_probe_evidence() {
  local runtime="$1" output="$2" name
  cp "$runtime/control.log" "$runtime/probe.json" "$output/"
  for name in send packet device server receive; do
    if [[ -f "$runtime/$name-control.log" ]]; then cp "$runtime/$name-control.log" "$output/"; fi
  done
}
