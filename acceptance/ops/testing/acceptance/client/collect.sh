#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
collection="$1"
commit="$2"
source_root="$3"
source "$(dirname "${BASH_SOURCE[0]}")/verify.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-inputs.sh"
for arch in amd64 arm64; do
  report="$collection/client-linux-$arch"
  [[ "$(cat "$report/status")" == passed ]] || exit 1
  [[ "$(sed -n 's/^target=//p' "$report/identity.txt")" == "linux/$arch" ]] || exit 1
  [[ "$(sed -n 's/^workflow_sha=//p' "$report/identity.txt")" == "$commit" ]] || exit 1
  client_verify_manifest "$report" "$collection/client-$arch-entries"
  (cd "$report"; shasum -a 256 --check evidence.sha256) > "$report/integrity-verification.log"
  client_verify_build "$report/network.test" "$report/test-build-observed.txt" "$arch"
  client_verify_build "$report/nhop-client" "$report/client-build-observed.txt" "$arch"
  for engine in xray sing-box; do
    [[ "$(sed -n 's/^target=//p' "$report/$engine-provenance.txt")" == "linux/$arch" ]] || exit 1
    cmp "$report/$engine-SHA256SUMS" "$report/$engine-observed.sha256"
  done
  for policy in tunnel block; do
    mode="$report/$policy"
    client_verify_inputs public "$mode"
    client_verify_public "$mode" "$policy"
    mode="$report/service/$policy"
    client_verify_inputs systemd "$mode"
    client_verify_systemd "$mode" "$policy"
  done
done
