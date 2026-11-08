#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/../../../testing/linux/fleet/environment.sh"
fleet_lab_environment
fleet_lab_owned
case "${1:-}" in build/tuic-regression/*) ;; *) exit 2 ;; esac
name="${1#build/tuic-regression/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
work="$lab_root/$1"
[[ -d "$work" && ! -L "$work" && "$(cat "$work/status")" == passed ]] || exit 2
(cd "$work"; shasum -a 256 --check --quiet evidence.sha256)
shasum -a 256 --check --quiet "$work/inputs.sha256"
mkdir "$work/native"
remote="/home/nhop/tuic-shutdown-$name"
[[ "$(fleet_lab_shell fleet-c hostname)" == lima-fleet-c ]]
[[ "$(fleet_lab_shell fleet-c uname -m)" == "$lab_guest_arch" ]]
fleet_lab_shell fleet-c mkdir -m 0700 "$remote"
for variant in control patched; do
  GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 GOWORK=off go -C "$work/$variant" test -c -trimpath -buildvcs=false \
    -ldflags=-buildid= -o "$work/native/$variant.test" ./tuic
  "$lab_lima" copy "$work/native/$variant.test" "fleet-c:$remote/$variant.test"
  expected="$(shasum -a 256 "$work/native/$variant.test" | awk '{print $1}')"
  actual="$(fleet_lab_shell fleet-c sha256sum "$remote/$variant.test")"
  [[ "${actual%% *}" == "$expected" ]]
done
{
  fleet_lab_shell fleet-c uname -a
  fleet_lab_shell fleet-c cat /proc/sys/kernel/random/boot_id
  shasum -a 256 "$work/native/"*.test "${BASH_SOURCE[0]}"
} > "$work/native/provenance.txt"
set +e
fleet_lab_shell fleet-c "$remote/control.test" -test.v -test.timeout=60s \
  -test.run='^TestServerShutdown$/early=false/cancel=false$' > "$work/native/control.log" 2>&1
control_result=$?
set -e
cat "$work/native/control.log"
[[ "$control_result" == 1 ]]
grep -Fq 'peer did not receive QUIC CONNECTION_CLOSE before UDP socket closed' "$work/native/control.log"
! grep -Eq 'panic:|DATA RACE' "$work/native/control.log"
fleet_lab_shell fleet-c "$remote/patched.test" -test.v -test.count=20 -test.timeout=120s \
  -test.run='^TestServerShutdown$' > "$work/native/patched.log" 2>&1
cat "$work/native/patched.log"
fleet_lab_shell fleet-c rm "$remote/control.test" "$remote/patched.test"
fleet_lab_shell fleet-c rmdir "$remote"
printf 'passed\n' > "$work/native/status"
(cd "$work/native"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/native/evidence.sha256"
printf 'Native TUIC shutdown regression: %s/native\n' "$work"
