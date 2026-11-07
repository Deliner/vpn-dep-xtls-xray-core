#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "${GITHUB_ACTIONS:-}" == true ]] || exit 2
[[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) exit 2 ;; esac
root="$PWD"
source "$root/ops/runtime/integrity.sh"
report="$root/build/client-acceptance"
mkdir -m 0700 "$report"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
{
  printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\n' "$arch" "${GITHUB_SHA:?}" "${GITHUB_RUN_ID:?}"
  uname -a
  lscpu
  go version
  cc --version
  dpkg-query --show nftables iproute2 dbus systemd-resolved ca-certificates
} > "$report/identity.txt"
for engine in xray sing-box; do
  output="build/runtime-candidates/client-$engine-linux-$arch"
  just candidate-build "$engine" linux "$arch" "$output" > "$report/$engine-build.log" 2>&1
  candidate_complete "$output"
  candidate_inventory "$output" > "$report/$engine-observed.sha256"
  cmp "$output/SHA256SUMS" "$report/$engine-observed.sha256"
  cp "$output/provenance.txt" "$report/$engine-provenance.txt"
  cp "$output/SHA256SUMS" "$report/$engine-SHA256SUMS"
done
CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go test -race -c -trimpath -buildvcs=false \
  -o "$report/network.test" ./internal/control/client/acceptance
go version -m "$report/network.test" > "$report/test-build.txt"
for policy in tunnel block; do
  mode="$report/$policy"
  mkdir -m 0700 "$mode" "$mode/bin"
  cp "$report/network.test" "$mode/network.test"
  cp ops/testing/client/kernel-guest.sh "$mode/guest.sh"
  cp ops/testing/client/resolved-guest.sh "$mode/resolved-guest.sh"
  cp "build/runtime-candidates/client-xray-linux-$arch/bin/xray" "$mode/bin/"
  cp "build/runtime-candidates/client-sing-box-linux-$arch/bin/"* "$mode/bin/"
  (cd "$mode"; sha256sum network.test guest.sh resolved-guest.sh bin/*) > "$mode/inputs.sha256"
  before="$(sha256sum /etc/passwd /etc/group /etc/ssl/certs/ca-certificates.crt /etc/resolv.conf)"
  result=0
  (cd "$mode"; sudo -n env GITHUB_ACTIONS=true GITHUB_REPOSITORY="$GITHUB_REPOSITORY" NHOP_CLIENT_CI="linux/$arch" \
    GORACE=atexit_sleep_ms=0 unshare --mount --net bash guest.sh public "$policy") > "$mode/guest.log" 2>&1 || result=$?
  for file in bus.log resolved.log; do
    if [[ -f "$mode/$file" ]]; then sudo -n chown "$(id -u):$(id -g)" "$mode/$file"; fi
  done
  [[ "$(sha256sum /etc/passwd /etc/group /etc/ssl/certs/ca-certificates.crt /etc/resolv.conf)" == "$before" ]]
  ((result == 0)) || { cat "$mode/guest.log"; exit "$result"; }
  ! grep -q 'WARNING: DATA RACE' "$mode/guest.log"
  (cd "$mode"; sha256sum --check inputs.sha256) > "$mode/input-integrity.log"
  printf 'passed\n' > "$mode/status"
done
printf 'passed\n' > "$report/status"
