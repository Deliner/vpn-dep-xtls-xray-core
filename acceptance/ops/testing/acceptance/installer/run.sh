#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "${GITHUB_ACTIONS:-}" == true ]] || exit 2
[[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core && "${RUNNER_ENVIRONMENT:-}" == github-hosted ]] || exit 2
[[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ && "${GITHUB_RUN_ID:-}" =~ ^[1-9][0-9]*$ ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) exit 2 ;; esac
root="$PWD"
source "$root/ops/testing/acceptance/installer/host.sh"
report="$root/build/installer-acceptance"
mkdir -m 0700 "$report"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
cp source.txt "$report/source.txt"
project="$(sed -n 's/^project_commit=//p' source.txt)"
[[ "$project" =~ ^[0-9a-f]{40}$ ]] || exit 2
printf 'target=linux/%s\nworkflow_sha=%s\nproject_commit=%s\nrun_id=%s\nmachine=%s\n' \
  "$arch" "$GITHUB_SHA" "$project" "$GITHUB_RUN_ID" "$(uname -m)" > "$report/identity.txt"
{
  uname -a
  lscpu
  go version
  systemctl --version
  dpkg-query --show iproute2 tcpdump curl ca-certificates
  [[ "$(cat /proc/1/comm)" == systemd ]]
} > "$report/environment.txt"
just test-race > "$report/race.log" 2>&1
just acceptance-installer-previous "$arch" "$report/previous" > "$report/previous-build.log" 2>&1
previous_sha="$(shasum -a 256 "$report/previous/bin/nhop-node" | awk '{print $1}')"
just lab-installer-package "$arch" "$report/package" "$report/previous/bin/nhop-node" "$previous_sha" > "$report/package.log" 2>&1
installer_host_clean > "$report/clean-before.txt"
installer_host_baseline > "$report/host-before.txt"
NHOP_SYSTEMD_ACCEPTANCE=owned-native-ci NHOP_NODE_CI="linux/$arch" bash "$report/package/payload/guest.sh" > "$report/guest.log" 2>&1
installer_host_clean > "$report/clean-after.txt"
installer_host_baseline > "$report/host-after.txt"
cmp "$report/host-before.txt" "$report/host-after.txt"
printf 'passed\n' > "$report/status"
