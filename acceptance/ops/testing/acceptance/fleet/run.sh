#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "${GITHUB_ACTIONS:-}" == true ]] || exit 2
[[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core && "${RUNNER_ENVIRONMENT:-}" == github-hosted ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) exit 2 ;; esac
root="$PWD"
report="$root/build/fleet-acceptance"
source ops/testing/acceptance/installer/host.sh
mkdir -m 0700 "$report" "$report/package" "$report/package/bin"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
cp source.txt "$report/source.txt"
{
  printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\n' "$arch" "${GITHUB_SHA:?}" "${GITHUB_RUN_ID:?}"
  uname -a
  lscpu
  go version
  cc --version
  systemctl --version
  [[ "$(cat /proc/1/comm)" == systemd ]]
} > "$report/identity.txt"
just test-race > "$report/race.log" 2>&1
CGO_ENABLED=1 go build -race -trimpath -buildvcs=false -ldflags=-buildid= -o "$report/package/bin/nhop-fleet" ./cmd/nhop-fleet
CGO_ENABLED=1 go test -race -c -trimpath -buildvcs=false -ldflags=-buildid= -o "$report/package/fleet.test" ./internal/control/fleet/acceptance
cp ops/testing/linux/fleet/{guest,context}.sh "$report/package/"
(cd "$report/package"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/inputs.sha256"
cp "$report/inputs.sha256" "$report/package/SHA256SUMS"
installer_host_clean > "$report/clean-before.txt"
installer_host_baseline > "$report/host-before.txt"
fleet_clean() {
  local path
  for path in /var/lib/nhop-fleet-agent /var/lib/nhop-fleet-service /etc/systemd/system/nhop-fleet-agent.service \
    /etc/systemd/system/nhop-fleet-agent.service.d /run/systemd/system/nhop-fleet-agent.service \
    /run/systemd/system/nhop-fleet-agent.service.d /etc/systemd/system/multi-user.target.wants/nhop-fleet-agent.service; do
    [[ ! -e "$path" && ! -L "$path" ]] || return 2
  done
  if pgrep -x nhop-fleet; then return 2; fi
}
fleet_clean
NHOP_FLEET_ACCEPTANCE=owned-native-ci NHOP_NODE_CI="linux/$arch" bash "$report/package/guest.sh" > "$report/guest.log" 2>&1
installer_host_clean > "$report/clean-after.txt"
fleet_clean
installer_host_baseline > "$report/host-after.txt"
cmp "$report/host-before.txt" "$report/host-after.txt"
just fleet-scale-test 1000 8 > "$report/scale.log" 2>&1
printf 'passed\n' > "$report/status"
