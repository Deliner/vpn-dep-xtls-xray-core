#!/usr/bin/env bash
# Real SOCKS port exhaustion inside a new netns, never the VM's main namespace.
set -euo pipefail
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
lab_environment
lab_owned
case "$1" in run-* ) ;; *) printf 'Use a packaged run name.\n' >&2; exit 2 ;; esac
case "$1" in *[!a-zA-Z0-9_-]* ) exit 2 ;; esac
[[ "$2" == /home/nhop/* ]] || { printf 'Use an explicit runtime path in the owned VM.\n' >&2; exit 2; }
lab_shell sudo -n unshare --net bash -s -- "$1" "$2" <<'GUEST'
set -euo pipefail
[[ "$(hostname)" == lima-payload && "$(id -u)" == 0 ]] || exit 2
bundle="/home/nhop/runs/$1/payload"
[[ -f "$2" && ! -L "$2" && -x "$2" ]] || exit 2
for test_name in TestSOCKSUDPAssociationPortsAreExclusive TestPlannedListenersSurviveEphemeralExhaustion; do
  [[ "$("$bundle/integration.test" -test.list "^$test_name$")" == "$test_name" ]] || {
    printf 'Packaged test binary lacks %s; create a fresh lab-payload run.\n' "$test_name" >&2; exit 2;
  }
done
ip link set lo up
sysctl -q -w 'net.ipv4.ip_local_port_range=45000 45031'
cd "$bundle/internal/control/integration"
NHOP_SOCKS_EXCLUSIVE=private-netns XRAY_BINARY="$2" "$bundle/integration.test" \
  -test.run '^Test(SOCKSUDPAssociationPortsAreExclusive|PlannedListenersSurviveEphemeralExhaustion)$' -test.v -test.timeout=30s
GUEST
