#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) printf 'Native Linux CPU does not match requested acceptance target.\n' >&2; exit 2 ;; esac
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
cd "$root"
report="$root/build/acceptance-results"
[[ ! -e "$report" && ! -L "$report" ]] || exit 2
mkdir -p "$report"
trap 'printf "failed\n" > "$report/status"' ERR
printf 'running\n' > "$report/status"
started="$(date +%s)"
printf '%s\n' "$started" > "$report/started-unix"
{
  date -u +%FT%TZ
  uname -a
  lscpu
  cat /etc/os-release
  printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\n' "$arch" "${GITHUB_SHA:-local}" "${GITHUB_RUN_ID:-local}"
  cat source.txt
} > "$report/host.txt"
for engine in xray sing-box; do
  output="build/runtime-candidates/$engine-linux-$arch"
  just candidate-build "$engine" linux "$arch" "$output" > "$report/$engine-build.log" 2>&1
  just candidate-audit "$output" > "$report/$engine-audit.log" 2>&1
  cp "$output/provenance.txt" "$report/$engine-provenance.txt"
  cp "$output/SHA256SUMS" "$report/$engine-SHA256SUMS"
done
xray_source="$root/build/runtime-candidates/xray-linux-$arch/source"
just dependency-test "$xray_source" ./transport/internet/masque/... > "$report/xray-masque-regressions.log" 2>&1
(cd "$xray_source/.."; sha256sum --check SHA256SUMS) > "$report/xray-regressions-integrity.log"
quic_source="$(go -C "$xray_source" list -m -f '{{.Dir}}' github.com/apernet/quic-go)"
GOFLAGS=-mod=readonly just dependency-test "$quic_source" . Test > "$report/quic-datagram-regressions.log" 2>&1
GOFLAGS=-mod=readonly just dependency-test "$quic_source" ./integrationtests/self '^TestNHOPConnectionDatagramBurst$' >> "$report/quic-datagram-regressions.log" 2>&1
go -C "$xray_source" mod verify > "$report/quic-module-integrity.log"
xray="$root/build/runtime-candidates/xray-linux-$arch/bin/xray"
singbox="$root/build/runtime-candidates/sing-box-linux-$arch/bin/sing-box"
NHOP_IPV6_ACCEPTANCE=1 just payload "$xray" "$singbox" Test "$report/payload" > "$report/payload.log" 2>&1
just payload-report "$report/payload" '1,3' "$started" "linux/$arch" '127.0.0.1' > "$report/verification.log" 2>&1
for scenario in ipv6-single ipv6-chain mixed-6-4-6 mixed-4-6-4; do
  hops=3
  addresses=::1
  case "$scenario" in ipv6-single) hops=1 ;; mixed-6-4-6) addresses=::1,127.0.0.1,::1 ;; mixed-4-6-4) addresses=127.0.0.1,::1,127.0.0.1 ;; esac
  just payload-report "$report/payload/$scenario" "$hops" "$started" "linux/$arch" "$addresses" >> "$report/verification.log" 2>&1
done
printf 'passed\n' > "$report/status"
(cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"
