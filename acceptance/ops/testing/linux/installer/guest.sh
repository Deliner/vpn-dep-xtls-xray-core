#!/usr/bin/env bash
# Explicit node systemd acceptance in the owned VM; never used by ordinary payload/check.
set -euo pipefail
umask 077
[[ "$(uname -s)" == Linux && "$(hostname)" == lima-payload && -d /run/systemd/system ]] || exit 2
bundle="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$bundle"
sha256sum --check SHA256SUMS
mkdir reports
printf 'running\n' > reports/status
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$bundle/reports/status"; fi' EXIT
started="$(date +%s)"
{
  uname -a
  cat /etc/os-release
  systemctl --version
  curl --version
} > reports/guest.txt
status=0
"$bundle/installer.test" -test.v -test.timeout=2m > reports/installer-unit.log 2>&1
"$bundle/node.test" -test.v -test.timeout=2m > reports/node-unit.log 2>&1
cd internal/control/integration
previous=""
[[ ! -f "$bundle/bin/previous-nhop-node" ]] || previous="$bundle/bin/previous-nhop-node"
if sudo -n env NHOP_SYSTEMD_ACCEPTANCE=private-lima NHOP_NODE_BINARY="$bundle/bin/nhop-node" \
  NHOP_PREVIOUS_NODE_BINARY="$previous" \
  NHOP_SYSTEMD_REPORT="$bundle/reports/installed.json" "$bundle/integration.test" \
  -test.run '^TestInstalledNodeLifecycle$' -test.v -test.timeout=20m > "$bundle/reports/payload.log" 2>&1; then
  printf 'passed\n' > "$bundle/reports/status"
else
  status=$?
  printf 'failed\n' > "$bundle/reports/status"
fi
cd "$bundle"
if ! grep -q '^--- PASS: TestInstalledNodeLifecycle' reports/payload.log || [[ ! -f reports/installed.json ]]; then
  status=1
  printf 'failed\n' > reports/status
fi
sudo -n journalctl -u vpn-nhop.service --no-pager --since "@$started" > reports/service.log
sudo -n chown -R "$(id -u):$(id -g)" reports
if ! sha256sum --check SHA256SUMS > reports/integrity-after.txt 2>&1; then
  status=1
  printf 'failed\n' > reports/status
fi
cat reports/payload.log
exit "$status"
