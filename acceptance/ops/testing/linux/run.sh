#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
lab_environment
lab_owned
mode="${1:-}"
shift
case "$mode" in
  payload) [[ $# == 2 ]] || exit 2 ;;
  installer) [[ $# == 0 || $# == 2 ]] || exit 2 ;;
  *) exit 2 ;;
esac
mkdir -p "$lab_data/runs"
run="$(mktemp -d "$lab_data/runs/run-XXXXXXXX")"
printf 'preparing\n' > "$run/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
run_name="$(basename "$run")"
if [[ "$mode" == payload ]]; then
  bash "$lab_root/ops/testing/linux/package.sh" "$lab_arch" "$1" "$2" "$run/package"
else
  bash "$lab_root/ops/testing/linux/installer-package.sh" "$lab_arch" "$run/package" "${1:-}" "${2:-}"
fi
lab_shell mkdir -p "runs/$run_name"
"$lab_lima" copy "$run/package/payload.tar.gz" "$lab_instance:/home/nhop/runs/$run_name/payload.tar.gz"
archive_sha="$(shasum -a 256 "$run/package/payload.tar.gz" | awk '{print $1}')"
status=0
if lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$run_name" "$archive_sha" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
cd "runs/$1"
printf '%s  payload.tar.gz\n' "$2" | sha256sum --check
tar -xzf payload.tar.gz
bash payload/guest.sh
GUEST
then
  printf 'passed\n' > "$run/status"
else
  status=$?
  printf 'failed\n' > "$run/status"
fi
if ! "$lab_lima" copy --recursive "$lab_instance:/home/nhop/runs/$run_name/payload/reports" "$run/reports"; then
  status=1
  printf 'failed\n' > "$run/status"
fi
cat "$run/guest.log"
printf '\nLinux evidence: %s\n' "$run"
exit "$status"
