#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/../../../testing/linux/environment.sh"
lab_environment
lab_owned
[[ $# == 2 ]] || exit 2
case "$1" in build/quic-mtu-regression/*) ;; *) exit 2 ;; esac
case "$2" in build/quic-mtu-native/*) ;; *) exit 2 ;; esac
input="${1#build/quic-mtu-regression/}" name="${2#build/quic-mtu-native/}"
for value in "$input" "$name"; do
  case "$value" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
done
[[ ! -L "$lab_root/build" && ! -L "$lab_root/build/quic-mtu-native" && ! -L "$lab_root/build/quic-mtu-regression" ]] || exit 2
source_run="$lab_root/$1"
[[ -d "$source_run" && ! -L "$source_run" && "$(cat "$source_run/status")" == passed ]] || exit 2
[[ -z "$(find "$source_run" -type l -print)" ]] || exit 2
mkdir -p "$lab_root/build/quic-mtu-native"
mkdir "$lab_root/$2"
work="$lab_root/$2"
printf 'running\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
recipe="$lab_root/ops/runtime/xray/quic"
source "$recipe/inputs.sh"
quic_regression_verify "$lab_root" "$source_run" "$work/source-evidence.sha256"
[[ "$(lab_shell hostname)" == lima-payload && "$(lab_shell uname -m)" == "$lab_guest_arch" ]]
mkdir "$work/experiment"
for variant in control patched; do
  GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 GOWORK=off go -C "$source_run/$variant" test -c -trimpath -buildvcs=false \
    -ldflags=-buildid= -o "$work/experiment/$variant.test" ./internal/ackhandler
done
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 GOWORK=off go -C "$source_run/patched" test -c -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$work/experiment/congestion.test" ./internal/congestion
cp "$recipe/linux.sh" "$work/experiment/guest.sh"
printf '%s\n' "$lab_guest_arch" > "$work/experiment/architecture"
{
  go version
  git -C "$lab_root" rev-parse HEAD
  printf 'source=%s\nCGO_ENABLED=0\n' "$1"
  shasum -a 256 "$recipe/native.sh" "$recipe/linux.sh" "$recipe/inputs.sh" "$source_run/evidence.sha256"
} > "$work/provenance.txt"
(cd "$work/experiment"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/input.sha256"
cp "$work/input.sha256" "$work/experiment/SHA256SUMS"
remote="/home/nhop/quic-mtu-$name"
lab_shell mkdir -m 0700 "$remote"
"$lab_lima" copy --recursive "$work/experiment" "$lab_instance:$remote/experiment"
result=0
lab_shell flock --nonblock /home/nhop/.payload.lock bash "$remote/experiment/guest.sh" > "$work/guest.log" 2>&1 || result=$?
"$lab_lima" copy --recursive "$lab_instance:$remote/experiment/reports" "$work/reports" || result=1
[[ -f "$work/reports/evidence.sha256" ]] || exit 1
(cd "$work/reports"; shasum -a 256 --check --quiet evidence.sha256) || result=1
((result == 0)) || exit "$result"
# The binaries remain available locally; remove only these verified executable copies from the VM.
lab_shell rm "$remote/experiment/control.test" "$remote/experiment/patched.test" "$remote/experiment/congestion.test"
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
printf 'Native Linux QUIC PMTU regression: %s\n' "$work"
