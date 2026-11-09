#!/usr/bin/env bash
set -euo pipefail
umask 077
bundle="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$bundle"
mode="${NHOP_FLEET_ACCEPTANCE:-private-lima}"
source "$bundle/context.sh"
fleet_execution_context
sha256sum --check SHA256SUMS
mkdir reports
printf 'running\n' > reports/status
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$bundle/reports/status"; fi' EXIT
stage="$(sudo -n mktemp -d /var/lib/nhop-fleet-acceptance-XXXXXXXX)"
printf '%s\n' "$stage" > reports/guest-stage.txt
sudo -n install -m 0700 -o 0 -g 0 bin/nhop-fleet "$stage/nhop-fleet"
sudo -n install -m 0700 -o 0 -g 0 fleet.test "$stage/fleet.test"
for name in nhop-fleet fleet.test; do
  source_file="$name"
  [[ "$name" != nhop-fleet ]] || source_file="bin/nhop-fleet"
  expected="$(sha256sum "$source_file" | awk '{print $1}')"
  observed="$(sudo -n sha256sum "$stage/$name" | awk '{print $1}')"
  [[ "$observed" == "$expected" ]] || exit 1
done
sudo -n mkdir -m 0700 "$stage/reports"
started="$(date +%s)"
{ uname -a; cat /etc/os-release; systemctl --version; } > reports/guest.txt
status=0
if sudo -n env NHOP_FLEET_ACCEPTANCE="$mode" NHOP_SYSTEMD_ACCEPTANCE="$mode" NHOP_FLEET_BINARY="$stage/nhop-fleet" \
  GITHUB_ACTIONS="${GITHUB_ACTIONS:-}" GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-}" \
  GITHUB_REPOSITORY_ID="${GITHUB_REPOSITORY_ID:-}" GITHUB_REPOSITORY_OWNER_ID="${GITHUB_REPOSITORY_OWNER_ID:-}" \
  RUNNER_ENVIRONMENT="${RUNNER_ENVIRONMENT:-}" GITHUB_EVENT_NAME="${GITHUB_EVENT_NAME:-}" GITHUB_REF="${GITHUB_REF:-}" \
  GITHUB_SHA="${GITHUB_SHA:-}" GITHUB_RUN_ID="${GITHUB_RUN_ID:-}" NHOP_NODE_CI="${NHOP_NODE_CI:-}" \
  GORACE=atexit_sleep_ms=0 \
  NHOP_FLEET_TEST_ROOT="$stage" NHOP_FLEET_REPORT="$stage/reports/fleet.json" \
  "$stage/fleet.test" -test.run '^TestNativeFleetLifecycle$' -test.v -test.timeout=20m > reports/fleet.log 2>&1; then
  printf 'passed\n' > reports/status
else
  status=$?
  printf 'failed\n' > reports/status
fi
sudo -n cp -a "$stage/reports/." reports/
sudo -n chown -R "$(id -u):$(id -g)" reports
sudo -n journalctl --no-pager -u nhop-fleet-agent.service -u vpn-nhop.service --since "@$started" > reports/services.log
if ! grep -q '^--- PASS: TestNativeFleetLifecycle' reports/fleet.log || [[ ! -f reports/fleet.json ]]; then status=1; fi
if ! sha256sum --check SHA256SUMS > reports/integrity-after.txt 2>&1; then status=1; fi
if ((status == 0)); then
  sudo -n rm -rf -- "$stage"
else
  printf 'failed\n' > reports/status
  printf 'Retained private guest stage for diagnosis: %s\n' "$stage"
fi
cat reports/fleet.log
exit "$status"
