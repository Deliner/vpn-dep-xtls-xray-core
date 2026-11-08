#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/../../linux/environment.sh"
lab_environment
lab_owned
[[ $# == 4 ]] || exit 2
probe="$(cd "$1" && pwd -P)"
case "$probe/" in "$lab_root/build/runtime-probes/"*) ;; *) exit 2 ;; esac
[[ "$(cat "$probe/status")" == diagnostic-built ]] || exit 2
[[ "$(sed -n 's/^target=//p' "$probe/provenance.txt" | head -1)" == "linux/$lab_arch" ]] || exit 2
mkdir -p "$lab_data/runs"
run="$(mktemp -d "$lab_data/runs/masque-netem-XXXXXXXX")"
printf 'preparing\n' > "$run/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
(cd "$probe"; shasum -a 256 --check SHA256SUMS) > "$run/probe-integrity.log"
mkdir "$run/experiment" "$run/experiment/bin"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false \
  -o "$run/experiment/network.test" ./internal/control/diagnostics/masque
cp "$probe/bin/xray" "$run/experiment/bin/xray"
cp "$probe/provenance.txt" "$probe/control.log" "$probe/probe.json" "$run/experiment/"
if [[ -f "$probe/send-control.log" ]]; then cp "$probe/send-control.log" "$run/experiment/"; fi
cp ops/testing/diagnostics/netem/guest.sh ops/testing/diagnostics/netem/namespace.sh "$run/experiment/"
printf '%s\n' "$2" > "$run/experiment/cycles"
printf '%s\n' "$3" > "$run/experiment/workers"
printf '%s\n' "$4" > "$run/experiment/packets"
{
  go version
  git rev-parse HEAD
  git status --porcelain
  find internal -name '*.go' -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
  shasum -a 256 go.mod go.sum ops/testing/diagnostics/netem/*.sh
} > "$run/experiment/source.txt"
(cd "$run/experiment"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$run/SHA256SUMS"
cp "$run/SHA256SUMS" "$run/experiment/"
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$run/experiment.tar.gz" -C "$run" experiment
digest="$(shasum -a 256 "$run/experiment.tar.gz" | awk '{print $1}')"
name="$(basename "$run")"
lab_shell mkdir -p "runs/$name"
"$lab_lima" copy "$run/experiment.tar.gz" "$lab_instance:/home/nhop/runs/$name/experiment.tar.gz"
result=0
if lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$name" "$digest" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
cd "runs/$1"
printf '%s  experiment.tar.gz\n' "$2" | sha256sum --check
tar -xzf experiment.tar.gz
cd experiment
bash guest.sh
GUEST
then result=0; else result=$?; fi
"$lab_lima" copy --recursive "$lab_instance:/home/nhop/runs/$name/experiment/reports" "$run/reports" || result=1
cat "$run/guest.log"
if [[ -f "$run/reports/evidence.sha256" ]]; then
  (cd "$run/reports"; shasum -a 256 --check evidence.sha256) > "$run/report-integrity.log" || result=1
else result=1; fi
if ((result == 0)); then
  printf 'passed\n' > "$run/status"
else printf 'failed\n' > "$run/status"; fi
printf 'MASQUE controlled network evidence: %s\n' "$run"
exit "$result"
