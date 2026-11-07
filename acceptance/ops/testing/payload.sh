#!/usr/bin/env bash
set -euo pipefail
[[ $# == 9 ]] || exit 2
absolute_binary() {
  [[ -f "$1" && -x "$1" ]] || { printf 'Missing executable: %s\n' "$1" >&2; return 2; }
  printf '%s/%s\n' "$(cd "$(dirname "$1")" && pwd -P)" "$(basename "$1")"
}
XRAY_BINARY="$(absolute_binary "$1")"
SINGBOX_BINARY="$(absolute_binary "$2")"
XRAY_CLIENT_BINARY="$(absolute_binary "$3")"
SINGBOX_CLIENT_BINARY="$(absolute_binary "$4")"
export XRAY_BINARY SINGBOX_BINARY XRAY_CLIENT_BINARY SINGBOX_CLIENT_BINARY
export XRAY_UPDATED_BINARY= SINGBOX_UPDATED_BINARY= NHOP_UPDATED_NODES="$9"
if [[ -n "$7" || -n "$8" || -n "$9" ]]; then
  [[ -n "$7" && -n "$8" && -n "$9" ]] || { printf 'Updated nodes require both updated runtimes.\n' >&2; exit 2; }
  XRAY_UPDATED_BINARY="$(absolute_binary "$7")"
  SINGBOX_UPDATED_BINARY="$(absolute_binary "$8")"
fi
mkdir -p "$6"
export NHOP_CONCURRENT_REPORT="$(cd "$6" && pwd -P)"
export NHOP_RUNTIME_LOG_DIR="$NHOP_CONCURRENT_REPORT/runtime-logs"
probe_status=0
XRAY_BINARY="$XRAY_CLIENT_BINARY" SING_BOX_BINARY="$SINGBOX_CLIENT_BINARY" CGO_ENABLED=1 \
  go test -count=1 -race -v -run '^TestPinnedRuntimeIPQuery$' ./internal/control/probe \
  > "$NHOP_CONCURRENT_REPORT/probe.log" 2>&1 || probe_status=$?
cat "$NHOP_CONCURRENT_REPORT/probe.log"
if ((probe_status != 0)); then exit "$probe_status"; fi
CGO_ENABLED=1 go test -count=1 -race -v -run "$5" ./internal/control/integration
