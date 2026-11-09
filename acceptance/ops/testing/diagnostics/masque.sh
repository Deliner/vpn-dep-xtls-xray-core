#!/usr/bin/env bash
# Diagnostic-only repeated synthetic payload in this checkout's private VM.
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/../linux/environment.sh"
lab_environment
lab_owned
[[ $# == 5 ]] || exit 2
repeats="$4"
[[ "$repeats" =~ ^[1-9][0-9]?$ || "$repeats" == 100 ]] || exit 2
[[ "$5" == 0 || "$5" == 1 ]] || exit 2
probe="$(cd "$3" && pwd -P)"
case "$probe/" in "$lab_root/build/runtime-probes/"*) ;; *) exit 2 ;; esac
[[ "$(cat "$probe/status")" == diagnostic-built ]] || exit 2
[[ "$(sed -n 's/^target=//p' "$probe/provenance.txt" | head -1)" == "linux/$lab_arch" ]] || exit 2
(cd "$probe"; shasum -a 256 --check SHA256SUMS)
mkdir -p "$lab_data/runs"
run="$(mktemp -d "$lab_data/runs/masque-XXXXXXXX")"
printf 'preparing\n' > "$run/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
bash "$lab_root/ops/testing/linux/package.sh" "$lab_arch" "$1" "$2" "$run/package" > "$run/package.log" 2>&1
mkdir "$run/experiment" "$run/experiment/probe"
cp "$run/package/payload.tar.gz" "$run/experiment/"
cp "$probe/bin/xray" "$run/experiment/probe/xray"
cp "$probe/provenance.txt" "$probe/control.log" "$run/experiment/probe/"
if [[ -f "$probe/server-control.log" ]]; then cp "$probe/server-control.log" "$run/experiment/probe/"; fi
cp "$lab_root/ops/testing/diagnostics/masque-guest.sh" "$run/experiment/guest.sh"
printf '%s\n' "$repeats" > "$run/experiment/repeats"
printf '%s\n' "$5" > "$run/experiment/debug-level"
(cd "$run/experiment"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$run/SHA256SUMS"
cp "$run/SHA256SUMS" "$run/experiment/SHA256SUMS"
COPYFILE_DISABLE=1 tar -czf "$run/experiment.tar.gz" -C "$run" experiment
archive_sha="$(shasum -a 256 "$run/experiment.tar.gz" | awk '{print $1}')"
run_name="$(basename "$run")"
lab_shell mkdir -p "runs/$run_name"
"$lab_lima" copy "$run/experiment.tar.gz" "$lab_instance:/home/nhop/runs/$run_name/experiment.tar.gz"
status=0
if lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$run_name" "$archive_sha" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
cd "runs/$1"
printf '%s  experiment.tar.gz\n' "$2" | sha256sum --check
tar -xzf experiment.tar.gz
bash experiment/guest.sh
GUEST
then
  printf 'passed\n' > "$run/status"
else
  status=$?
  printf 'failed\n' > "$run/status"
fi
if ! "$lab_lima" copy --recursive "$lab_instance:/home/nhop/runs/$run_name/experiment/reports" "$run/reports"; then
  status=1
  printf 'failed\n' > "$run/status"
fi
cat "$run/guest.log"
printf '\nMASQUE diagnostic evidence: %s\n' "$run"
exit "$status"
