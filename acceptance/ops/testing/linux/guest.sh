#!/usr/bin/env bash
# Packaged implementation of just lab-payload; invoked only inside its Linux VM.
set -euo pipefail
umask 077
[[ "$(uname -s)" == Linux ]] || { printf 'Linux guest required.\n' >&2; exit 2; }
bundle="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$bundle"
sha256sum --check SHA256SUMS
mkdir reports
{
  uname -a
  cat /etc/os-release
  ldd --version
  curl --version
} > reports/guest.txt
status=0
for probe_suite in probe ipcheck; do
  if XRAY_BINARY="$bundle/bin/xray/xray" SING_BOX_BINARY="$bundle/bin/sing-box/sing-box" \
    "$bundle/$probe_suite.test" -test.v -test.timeout=1m > "$bundle/reports/$probe_suite.log" 2>&1; then
    :
  else
    status=$?
  fi
done
cd internal/control/integration
export NHOP_RUNTIME_LOG_DIR="$bundle/reports/runtime-logs"
if XRAY_BINARY="$bundle/bin/xray/xray" SINGBOX_BINARY="$bundle/bin/sing-box/sing-box" \
  NHOP_IPV6_ACCEPTANCE=1 NHOP_CONCURRENT_REPORT="$bundle/reports" "$bundle/integration.test" -test.v -test.timeout=10m > "$bundle/reports/payload.log" 2>&1; then
  :
else
  status=$?
fi
cd "$bundle"
if ((status == 0)); then
  printf 'passed\n' > reports/status
else
  printf 'failed\n' > reports/status
fi
if ! sha256sum --check SHA256SUMS > reports/integrity-after.txt 2>&1; then
  status=1
  printf 'failed\n' > reports/status
fi
cat reports/probe.log reports/ipcheck.log reports/payload.log
exit "$status"
