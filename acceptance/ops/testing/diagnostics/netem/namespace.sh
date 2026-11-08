#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ "$(uname -s)" == Linux && "$(hostname)" == lima-payload && "$(id -u)" == 0 ]] || exit 2
for namespace in net mnt pid; do
  variable="NHOP_MASQUE_HOST_$namespace"
  [[ -n "${!variable:-}" && "$(readlink "/proc/self/ns/$namespace")" != "${!variable}" ]] || {
    printf 'Missing private %s namespace; refusing network changes.\n' "$namespace" >&2
    exit 2
  }
done
[[ "$PWD" == /home/nhop/runs/masque-netem-*/experiment ]] || exit 2
mount --make-rprivate /
mkdir -p /run/netns
mount -t tmpfs nhop-masque-netem /run/netns
ip netns add nhop-masque-exit
ip link set lo up
ip link add nhop-link type veth peer name nhop-peer
ip link set nhop-peer netns nhop-masque-exit
ip address add 198.51.100.1/30 dev nhop-link
ip link set nhop-link up
ip netns exec nhop-masque-exit ip link set lo up
ip netns exec nhop-masque-exit ip link set nhop-peer name nhop-link
ip netns exec nhop-masque-exit ip address add 198.51.100.2/30 dev nhop-link
ip netns exec nhop-masque-exit ip link set nhop-link up
ip netns exec nhop-masque-exit ip address add 198.18.0.2/32 dev lo
# A passing test cannot silently use H2 instead of H3.
nft -f - <<'FILTER'
add table inet nhop_masque_test
add chain inet nhop_masque_test output { type filter hook output priority 0; policy accept; }
add rule inet nhop_masque_test output tcp dport 24443 counter reject
FILTER
mkdir -m 0700 reports
{
  date -u +%FT%TZ
  uname -a
  tc -V
  printf 'diagnostic_only=true\nloss_seed=none\n'
  for namespace in net mnt pid; do readlink "/proc/self/ns/$namespace"; done
  sha256sum network.test bin/xray
} > reports/identity.txt
capture=
finish() {
  local result=$?
  trap - EXIT
  if [[ -n "$capture" ]]; then
    kill -INT "$capture" 2>/dev/null || true
    wait "$capture" || result=1
  fi
  [[ -f reports/link.pcap && "$(stat -c %s reports/link.pcap)" -gt 24 ]] || result=1
  nft list table inet nhop_masque_test > reports/h3-only-firewall.txt || result=1
  if ((result == 0)); then printf 'passed\n' > reports/status; else printf 'failed\n' > reports/status; fi
  exit "$result"
}
trap finish EXIT
tcpdump -nn -U -i nhop-link -s 0 -w reports/link.pcap 'udp port 24443 or icmp' > reports/capture.log 2>&1 &
capture=$!
for attempt in $(seq 1 100); do
  if grep -q 'listening on' reports/capture.log; then break; fi
  kill -0 "$capture"
  sleep 0.05
done
grep -q 'listening on' reports/capture.log
export NHOP_MASQUE_NETEM_TEST=private-netns NHOP_MASQUE_REPORT="$PWD/reports" XRAY_BINARY="$PWD/bin/xray"
./network.test -test.run '^TestMASQUENetworkFaults$' -test.v -test.timeout=12m > reports/experiment.log 2>&1
cat reports/experiment.log
