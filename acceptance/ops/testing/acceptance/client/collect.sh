#!/usr/bin/env bash
set -euo pipefail
[[ $# == 3 ]] || exit 2
collection="$1"
commit="$2"
source_root="$3"
for arch in amd64 arm64; do
  report="$collection/client-linux-$arch"
  [[ "$(cat "$report/status")" == passed ]] || exit 1
  [[ "$(sed -n 's/^target=//p' "$report/identity.txt")" == "linux/$arch" ]] || exit 1
  [[ "$(sed -n 's/^workflow_sha=//p' "$report/identity.txt")" == "$commit" ]] || exit 1
  (cd "$report"; shasum -a 256 --check evidence.sha256) > "$report/integrity-verification.log"
  go version -m "$report/network.test" > "$report/test-build-observed.txt"
  grep -q -- '-race=true' "$report/test-build-observed.txt"
  grep -Fq $'\tbuild\tGOOS=linux' "$report/test-build-observed.txt"
  grep -Fq $'\tbuild\tGOARCH='"$arch" "$report/test-build-observed.txt"
  for engine in xray sing-box; do
    [[ "$(sed -n 's/^target=//p' "$report/$engine-provenance.txt")" == "linux/$arch" ]] || exit 1
    cmp "$report/$engine-SHA256SUMS" "$report/$engine-observed.sha256"
  done
  for policy in tunnel block; do
    mode="$report/$policy"
    [[ "$(cat "$mode/status")" == passed ]] || exit 1
    for name in network.test guest.sh resolved-guest.sh bin/xray bin/sing-box bin/libcronet.so; do
      [[ -f "$mode/$name" && ! -L "$mode/$name" && -s "$mode/$name" ]] || exit 1
      awk -v name="$name" '$2 == name { found++ } END { exit found != 1 }' "$mode/inputs.sha256"
    done
    [[ "$(wc -l < "$mode/inputs.sha256" | tr -d ' ')" == 6 ]] || exit 1
    (cd "$mode"; shasum -a 256 --check inputs.sha256) >> "$report/integrity-verification.log"
    cmp "$mode/network.test" "$report/network.test"
    cmp "$mode/guest.sh" "$source_root/ops/testing/client/kernel-guest.sh"
    cmp "$mode/resolved-guest.sh" "$source_root/ops/testing/client/resolved-guest.sh"
    for name in xray sing-box libcronet.so; do
      engine=sing-box
      if [[ "$name" == xray ]]; then engine=xray; fi
      digest="$(awk -v name="bin/$name" '$2 == name { print $1 }' "$report/$engine-SHA256SUMS")"
      [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || exit 1
      printf '%s  %s\n' "$digest" "$mode/bin/$name" | shasum -a 256 --check >> "$report/integrity-verification.log"
    done
    grep -q "native client acceptance: linux/$arch; private mount and network namespaces" "$mode/guest.log"
    grep -q "client policy: IPv6=$policy" "$mode/guest.log"
    ! grep -q 'WARNING: DATA RACE' "$mode/guest.log"
    for failure in graceful native controller-term controller-kill reconnect-native reconnect-frontend; do
      expected="--- PASS: TestKernelPublicClientAndControllerRecovery/$failure "
      awk -v expected="$expected" 'index($0, expected) { found=1 } END { exit !found }' "$mode/guest.log"
    done
    for phase in direct tunnel blocked; do grep -q "system DNS $phase:" "$mode/guest.log"; done
    awk '/reconnect direct UDP: attempts=[0-9]+ delivered=0; same firewall lease retained/ { count++ } END { exit count != 2 }' "$mode/guest.log"
  done
done
