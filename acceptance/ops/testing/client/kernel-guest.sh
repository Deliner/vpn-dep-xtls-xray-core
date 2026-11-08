#!/usr/bin/env bash
set -euo pipefail
[[ "$(uname -s)" == Linux && "$(id -u)" == 0 ]] || exit 2
if [[ "$(hostname)" != lima-payload ]]; then
  [[ "${GITHUB_ACTIONS:-}" == true && "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core ]] || exit 2
  case "${NHOP_CLIENT_CI:-}/$(uname -m)" in linux/amd64/x86_64|linux/arm64/aarch64) ;; *) exit 2 ;; esac
fi
[[ "$(readlink /proc/self/ns/net)" != "$(readlink /proc/1/ns/net)" ]] || exit 2
[[ "$(readlink /proc/self/ns/mnt)" != "$(readlink /proc/1/ns/mnt)" ]] || exit 2
case "${1:-policy}" in
  policy) pattern='^TestKernelKillSwitchDeniesDirectAndEstablishedEgress$' ;;
  tun) pattern='^TestKernelTUNTrafficDNSAndEngineFailure$' ;;
  resolver) pattern='^TestKernelResolvedPrivateLinkLifecycle$' ;;
  session) pattern='^TestKernelSessionStartStopAndFailures$' ;;
  public) pattern='^TestKernelPublicClientAndControllerRecovery$' ;;
  systemd) pattern='^TestKernelSystemdManagedRecovery$' ;;
  protocols) pattern='^TestKernelAllProtocolTUN$' ;;
  *) exit 2 ;;
esac
case "${2:-tunnel}" in tunnel|block) export NHOP_CLIENT_IPV6="${2:-tunnel}" ;; *) exit 2 ;; esac
# Names and veth interfaces exist only inside this private mount/network pair.
mount --make-rprivate /
mkdir -p /run/netns
mount -t tmpfs nhop-client-test /run/netns
ip netns add nhop-client-inner
client_daemons=()
client_guest_cleanup() {
  local result=$? pid
  trap - EXIT
  if [[ -n "${NHOP_CLIENT_SYSTEMD_UNIT:-}" ]]; then
    if ! client_systemd_cleanup; then result=1; fi
  fi
  ip netns pids nhop-client-inner | xargs -r kill || true
  # Reap the actual children before logs are hashed by the caller.
  for pid in "${client_daemons[@]}"; do wait "$pid" || true; done
  ip netns delete nhop-client-inner || true
  if [[ -n "${NHOP_CLIENT_SYSTEMD_ROOT:-}" ]]; then
    if ! client_systemd_remove_fixture; then result=1; fi
  fi
  exit "$result"
}
trap client_guest_cleanup EXIT
ip link set lo up
ip link add underlay type veth peer name client
ip link set client netns nhop-client-inner
ip address add 198.51.100.7/24 dev underlay
ip -6 address add 2001:db8:ffff::1/64 dev underlay nodad
ip link set underlay up
ip netns exec nhop-client-inner ip link set lo up
ip netns exec nhop-client-inner ip address add 198.51.100.8/24 dev client
ip netns exec nhop-client-inner ip -6 address add 2001:db8:ffff::2/64 dev client nodad
ip netns exec nhop-client-inner ip link set client up
ip netns exec nhop-client-inner ip route add default via 198.51.100.7
ip netns exec nhop-client-inner ip -6 route add default via 2001:db8:ffff::1
ip address add 198.51.100.9/32 dev lo
ip address add 198.51.100.11/32 dev lo
ip address add 198.18.0.2/32 dev lo
ip -6 address add 2001:db8:feed::2/128 dev lo nodad
if [[ "${1:-}" == resolver ]]; then
  # A separate resolved instance must never reach the guest's real runtime state.
  mount -t tmpfs -o mode=0755 nhop-resolved-test /run/systemd/resolve
  chown systemd-resolve:systemd-resolve /run/systemd/resolve
  ip link add nhopdns type dummy
  ip address add 172.30.255.1/30 dev nhopdns
  ip link set nhopdns up
  export NHOP_CLIENT_RESOLVED_TEST=private-netns
fi
if [[ "${1:-}" == session || "${1:-}" == public || "${1:-}" == systemd || "${1:-}" == protocols ]]; then
  if [[ "${1:-}" == systemd ]]; then source ./systemd-guest.sh; fi
  source ./resolved-guest.sh
  client_private_resolved "$1"
fi
if [[ "${1:-}" == public || "${1:-}" == systemd || "${1:-}" == protocols ]]; then
  ip netns exec nhop-client-inner resolvectl dns client 198.51.100.7
  ip netns exec nhop-client-inner resolvectl default-route client yes
  ip netns exec nhop-client-inner resolvectl dnsovertls client no
  ip netns exec nhop-client-inner resolvectl llmnr client no
  ip netns exec nhop-client-inner resolvectl mdns client no
  mkdir -p /run/nhop-client
  if [[ "${1:-}" == systemd ]]; then
    mount --bind "$NHOP_CLIENT_SYSTEMD_ROOT/control" /run/nhop-client
  else
    mount -t tmpfs -o mode=0711 nhop-client-control-test /run/nhop-client
  fi
  export NHOP_CLIENT_PUBLIC_TEST=private-netns
fi
if [[ "${1:-}" == systemd ]]; then
  client_systemd_prepare
fi
test_timeout=2m
if [[ "${1:-}" == protocols ]]; then
  export NHOP_CLIENT_PROTOCOL_TEST=private-netns NHOP_CLIENT_BINARY="$PWD/bin/nhop-client"
  test_timeout=10m
fi
NHOP_CLIENT_KERNEL_TEST=private-netns NHOP_CLIENT_RUNTIME="$PWD/bin" ./network.test \
  -test.run "$pattern" -test.v -test.timeout="$test_timeout"
case "${1:-}" in resolver|session|public|systemd|protocols) ;; *) ip netns exec nhop-client-inner nft -j list table inet nhop_client ;; esac
if [[ "${1:-}" == session || "${1:-}" == public || "${1:-}" == systemd || "${1:-}" == protocols ]]; then cat bus.log resolved.log; fi
