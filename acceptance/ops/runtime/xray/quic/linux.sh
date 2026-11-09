#!/usr/bin/env bash
set -euo pipefail
umask 077
cd "$(dirname "${BASH_SOURCE[0]}")"
[[ "$(id -u)" != 0 && "$(hostname)" == lima-payload ]]
[[ "$(uname -m)" == "$(cat architecture)" ]]
sha256sum --check SHA256SUMS
mkdir -m 0700 reports
finish() {
  local result=$?
  trap - EXIT
  sha256sum --check SHA256SUMS > reports/input-after.log || result=1
  if ((result == 0)); then printf 'passed\n' > reports/status; else printf 'failed\n' > reports/status; fi
  (cd reports; find . -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > reports/evidence.sha256
  exit "$result"
}
trap finish EXIT
{
  uname -a
  cat /proc/sys/kernel/random/boot_id
  printf 'uid=%s\n' "$(id -u)"
  sha256sum control.test patched.test congestion.test
} > reports/identity.txt
set +e
./control.test -test.v -test.timeout=60s -test.run='^TestMTUProbe' > reports/control.log 2>&1
control_result=$?
set -e
printf '%s\n' "$control_result" > reports/control.exit
[[ "$control_result" == 1 ]]
grep -Fq 'PMTU probe loss leaked into extended congestion control' reports/control.log
grep -Fq 'PMTU-only loss triggered extended congestion control' reports/control.log
grep -Fq -- '--- PASS: TestMTUProbeACKStillContributesToExtendedDelivery' reports/control.log
! grep -Eq 'panic:|DATA RACE' reports/control.log
./patched.test -test.v -test.count=20 -test.timeout=120s -test.run='^TestMTUProbe' > reports/patched.log 2>&1
./patched.test -test.v -test.timeout=120s > reports/ackhandler.log 2>&1
./congestion.test -test.v -test.timeout=120s > reports/congestion.log 2>&1
