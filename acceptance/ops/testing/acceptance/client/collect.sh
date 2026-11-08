#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
collection="$1"
commit="$2"
source_root="$3"
source "$(dirname "${BASH_SOURCE[0]}")/verify.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-inputs.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-protocols.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-boot.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-registration.sh"
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
  for kind in install boot-order; do
    client_verify_build "$report/$kind.test" "$report/$kind-build-observed.txt" "$arch"
    mode="$report/registration/$kind"
    client_verify_inputs "$kind" "$mode"
    client_verify_registration "$kind" "$mode"
  done
  for policy in tunnel block; do
    mode="$report/boot/$policy"
    client_verify_inputs boot "$mode"
    client_verify_boot "$mode" "$policy"
    mode="$report/$policy"
    client_verify_inputs public "$mode"
    client_verify_public "$mode" "$policy"
    mode="$report/service/$policy"
    client_verify_inputs systemd "$mode"
    client_verify_systemd "$mode" "$policy"
    mode="$report/installed/$policy"
    client_verify_inputs installed "$mode"
    just acceptance-client-installed-verify "$mode" "$policy"
    mode="$report/protocols/$policy"
    client_verify_inputs protocols "$mode"
    client_verify_protocols "$mode" "$policy"
  done
done
