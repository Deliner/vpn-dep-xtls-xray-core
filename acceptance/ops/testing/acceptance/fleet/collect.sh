#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
collection="$1" commit="$2" source_root="$3"
source "$(dirname "${BASH_SOURCE[0]}")/verify.sh"
source "$source_root/ops/testing/acceptance/client/verify-inputs.sh"
source "$source_root/ops/testing/acceptance/client/release.sh"
for arch in amd64 arm64; do
  report="$collection/fleet-linux-$arch"
  client_verify_manifest "$report" "$collection/fleet-$arch-entries"
  (cd "$report"; shasum -a 256 --check evidence.sha256) > "$collection/fleet-$arch-integrity.log"
  [[ "$(cat "$report/status")" == passed && "$(cat "$report/package/reports/status")" == passed ]]
  grep -Fxq "target=linux/$arch" "$report/identity.txt"
  grep -Fxq "workflow_sha=$commit" "$report/identity.txt"
  cmp "$report/source.txt" "$source_root/source.txt"
  cmp "$report/host-before.txt" "$report/host-after.txt"
  cmp "$report/clean-before.txt" "$report/clean-after.txt"
  cmp "$report/inputs.sha256" "$report/package/SHA256SUMS"
  (cd "$report/package"; shasum -a 256 --check SHA256SUMS) >> "$collection/fleet-$arch-integrity.log"
  for name in guest context; do cmp "$report/package/$name.sh" "$source_root/ops/testing/linux/fleet/$name.sh"; done
  fleet_verify_build "$report/package/bin/nhop-fleet" "$arch" "$collection/fleet-$arch-build.txt"
  fleet_verify_build "$report/package/fleet.test" "$arch" "$collection/fleet-$arch-test-build.txt"
  fleet_verify_report "$report/package/reports/fleet.json" "$arch"
  digest="$(shasum -a 256 "$report/package/bin/nhop-fleet" | awk '{print $1}')"
  jq -e --arg hash "$digest" '.agent_sha256 == $hash and .supervisor_sha256 == $hash' "$report/package/reports/fleet.json" > /dev/null
  for name in xray sing-box libcronet.so; do
    digest="$(shasum -a 256 "$report/package/reports/runtime/$name" | awk '{print $1}')"
    jq -e --arg name "$name" --arg hash "$digest" '.runtime_sha256[$name] == $hash' "$report/package/reports/fleet.json" > /dev/null
  done
  client_verify_release_runtime "$source_root/ops/release/runtime" "$arch" "$report/package/reports/runtime/xray" \
    "$report/package/reports/runtime/sing-box" "$report/package/reports/runtime/libcronet.so" >> "$collection/fleet-$arch-integrity.log"
  log="$report/package/reports/fleet.log"
  for phase in initial update activation-rollback controller-outage agent-crash revocation; do
    awk -v p="--- PASS: TestNativeFleetLifecycle/$phase (" 'index($0,p) {n++} END {exit n!=1}' "$log"
  done
  awk '/^--- PASS: TestNativeFleetLifecycle \(/ {n++} /^PASS$/ {p++} /--- (FAIL|SKIP):|WARNING: DATA RACE/ {bad=1} END {exit bad || n!=1 || p!=1}' "$log"
  grep -q 'Fleet HTTPS capacity PASS: nodes=1000 workers=8 revoked=100 ' "$report/scale.log"
  awk '/WARNING: DATA RACE|^FAIL|--- (FAIL|SKIP):/ {bad=1} END {exit bad}' "$report/scale.log" "$report/race.log" "$report/package/reports/services.log"
done
