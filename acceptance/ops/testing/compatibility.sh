#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
official_xray="$root/build/xray-test/xray"
official_singbox="$root/build/sing-box-test/sing-box"
candidate_xray="$1"
candidate_singbox="$2"
for binary in "$official_xray" "$official_singbox" "$candidate_xray" "$candidate_singbox"; do
  [[ -f "$binary" && -x "$binary" ]] || { printf 'Missing runtime executable: %s; use just payload-setup/candidate-build.\n' "$binary" >&2; exit 2; }
done
if cmp -s "$official_xray" "$candidate_xray" || cmp -s "$official_singbox" "$candidate_singbox"; then
  printf 'Compatibility requires distinct official/candidate bytes for both engines.\n' >&2
  exit 2
fi
[[ ! -e "$3" && ! -L "$3" && -d "$(dirname "$3")" ]] || { printf 'Use a new report directory with an existing parent.\n' >&2; exit 2; }
report="$(cd "$(dirname "$3")" && pwd -P)/$(basename "$3")"
case "$report" in "$root/build/"*) ;; *) printf 'Compatibility reports must stay under build/.\n' >&2; exit 2 ;; esac
mkdir "$report"
printf 'running\n' > "$report/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi' EXIT
status=0
for stage in official-clients candidate-clients exit-updated relay-and-exit-updated; do
  mkdir "$report/$stage"
  pattern=Test
  hops=1,3
  server_xray="$official_xray"; server_singbox="$official_singbox"
  client_xray="$official_xray"; client_singbox="$official_singbox"
  updated_xray= updated_singbox= updated_nodes=
  case "$stage" in
    official-clients) server_xray="$candidate_xray"; server_singbox="$candidate_singbox" ;;
    candidate-clients) client_xray="$candidate_xray"; client_singbox="$candidate_singbox" ;;
    *)
      updated_xray="$candidate_xray"; updated_singbox="$candidate_singbox"
      updated_nodes=node2
      hops=3
      [[ "$stage" != relay-and-exit-updated ]] || updated_nodes=node1,node2
      pattern='^(TestProtocolMatrixPayload|TestAllProtocolsConcurrent|TestMixedEngineRelayFailure)$/^(mixed-three-hop|3-hop)$'
      ;;
  esac
  started="$(date +%s)"
  if just payload-runtimes "$server_xray" "$server_singbox" "$client_xray" "$client_singbox" \
    "$pattern" "$report/$stage" "$updated_xray" "$updated_singbox" "$updated_nodes" > "$report/$stage/payload.log" 2>&1 && \
    just payload-report "$report/$stage" "$hops" "$started" >> "$report/$stage/payload.log" 2>&1; then
    printf 'passed\n' > "$report/$stage/status"
  else
    printf 'failed\n' > "$report/$stage/status"
    status=1
  fi
  printf '%s: %s (%s)\n' "$stage" "$(cat "$report/$stage/status")" "$report/$stage/payload.log"
done
if ((status == 0)); then printf 'passed\n' > "$report/status"; fi
exit "$status"
