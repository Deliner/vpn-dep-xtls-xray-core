#!/usr/bin/env bash
# Repeated UDP/443 probes use a real low-port canary only in an isolated guest netns.
set -euo pipefail
[[ $# == 3 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
lab_environment
lab_owned
case "$1" in run-* ) ;; *) printf 'Use a packaged run name.\n' >&2; exit 2 ;; esac
case "$1" in *[!a-zA-Z0-9_-]* ) exit 2 ;; esac
for binary in "$2" "$3"; do
  [[ "$binary" == /home/nhop/* ]] || { printf 'Use runtime paths in the owned VM.\n' >&2; exit 2; }
done
lab_shell sudo -n unshare --net bash -s -- "$1" "$2" "$3" <<'GUEST'
set -euo pipefail
[[ "$(hostname)" == lima-payload && "$(id -u)" == 0 ]] || exit 2
bundle="/home/nhop/runs/$1/payload"
for binary in "$2" "$3"; do [[ -f "$binary" && ! -L "$binary" && -x "$binary" ]] || exit 2; done
for test_name in TestUDP443RemainsBlocked TestMissingRoutingRulesFailClosed; do
  [[ "$("$bundle/integration.test" -test.list "^$test_name$")" == "$test_name" ]] || {
    printf 'Packaged test binary lacks %s; create a fresh lab-payload run.\n' "$test_name" >&2; exit 2;
  }
done
ip link set lo up
ip address add 198.18.0.2/32 dev lo
cd "$bundle/internal/control/integration"
NHOP_ROUTING_ISOLATED=private-netns XRAY_BINARY="$2" SINGBOX_BINARY="$3" \
  LD_LIBRARY_PATH="$(dirname "$3")" "$bundle/integration.test" \
  -test.run '^(TestUDP443RemainsBlocked|TestMissingRoutingRulesFailClosed)$' -test.v -test.timeout=4m
GUEST
