#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ "$(uname -s)" == Linux && "$(hostname)" == lima-payload ]] || exit 2
experiment="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$experiment"
sha256sum --check SHA256SUMS
tar -xzf payload.tar.gz
(cd payload; sha256sum --check SHA256SUMS)
mkdir reports
repeats="$(cat repeats)"
[[ "$repeats" =~ ^[1-9][0-9]?$ || "$repeats" == 100 ]] || exit 2
debug="$(cat debug-level)"
[[ "$debug" == 0 || "$debug" == 1 ]] || exit 2
{
  date -u +%FT%TZ
  uname -a
  printf 'planned_repeats=%s\ndiagnostic_only=true\n' "$repeats"
  printf 'runtime_debug=%s\n' "$debug"
  sha256sum probe/xray payload/bin/sing-box/*
} > reports/identity-before.txt
cat /proc/net/snmp > reports/snmp-before.txt
status=0
for ((iteration=1; iteration<=repeats; iteration++)); do
  output="$experiment/reports/repeat-$iteration"
  mkdir "$output"
  if (cd payload/internal/control/integration; \
    XRAY_BINARY="$experiment/probe/xray" SINGBOX_BINARY="$experiment/payload/bin/sing-box/sing-box" \
    NHOP_PAYLOAD_DEBUG="$debug" NHOP_RUNTIME_LOG_DIR="$output/runtime-logs" \
    "$experiment/payload/integration.test" -test.v -test.timeout=5m -test.run '^TestAllProtocolClientRestarts$') > "$output/payload.log" 2>&1; then
    printf 'passed\n' > "$output/status"
  else
    status=$?
    printf 'failed\n' > "$output/status"
  fi
  cat "$output/payload.log"
  printf 'Completed repeat %s/%s: %s\n' "$iteration" "$repeats" "$(cat "$output/status")"
  if ((status != 0)); then break; fi
done
cat /proc/net/snmp > reports/snmp-after.txt
if ! sha256sum --check SHA256SUMS > reports/integrity-after.txt 2>&1; then status=1; fi
if ! (cd payload; sha256sum --check SHA256SUMS) > reports/payload-integrity-after.txt 2>&1; then status=1; fi
if ((status == 0)); then printf 'passed\n' > reports/status; else printf 'failed\n' > reports/status; fi
exit "$status"
