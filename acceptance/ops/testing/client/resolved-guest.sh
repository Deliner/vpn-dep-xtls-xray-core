#!/usr/bin/env bash
# Sourced only after kernel-guest.sh establishes the private namespaces.
client_private_resolved() {
  # Interface indices in this namespace must not inherit host networkd ownership.
  mkdir -p /run/systemd/netif
  mkdir -p /run/nhop-session-test
  if [[ "${1:-}" == systemd ]]; then
    client_systemd_mounts
  else
    mount -t tmpfs -o mode=0755 nhop-resolved-test /run/systemd/resolve
    mount -t tmpfs -o mode=0755 nhop-netif-test /run/systemd/netif
    mount -t tmpfs -o mode=0755 nhop-session-test /run/nhop-session-test
  fi
  chown systemd-resolve:systemd-resolve /run/systemd/resolve
  if [[ "${1:-}" == public || "${1:-}" == systemd ]]; then client_private_config; fi
  export DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/nhop-session-test/bus
  cat > /run/nhop-session-test/bus.conf <<'CONFIG'
<busconfig><type>system</type><listen>unix:path=/run/nhop-session-test/bus</listen><auth>EXTERNAL</auth><policy context="default"><allow user="*"/><allow own="*"/><allow send_destination="*"/><allow receive_sender="*"/></policy></busconfig>
CONFIG
  ip netns exec nhop-client-inner dbus-daemon --nofork --nopidfile --config-file=/run/nhop-session-test/bus.conf > bus.log 2>&1 &
  client_daemons+=("$!")
  for attempt in $(seq 1 50); do [[ ! -S /run/nhop-session-test/bus ]] || break; sleep 0.1; done
  [[ -S /run/nhop-session-test/bus ]]
  ip netns exec nhop-client-inner /usr/lib/systemd/systemd-resolved > resolved.log 2>&1 &
  client_daemons+=("$!")
  for attempt in $(seq 1 50); do
    if ip netns exec nhop-client-inner resolvectl dns >/dev/null 2>&1; then return 0; fi
    sleep 0.1
  done
  cat bus.log resolved.log >&2
  return 1
}

client_private_config() {
  # Only this copied /etc receives account creation and the test TLS root.
  cp -a /etc /run/nhop-session-test/etc
  mount --bind /run/nhop-session-test/etc /etc
  rm /etc/resolv.conf
  printf 'nameserver 127.0.0.53\noptions timeout:1 attempts:1\n' > /etc/resolv.conf
  [[ -d /etc/systemd && ! -L /etc/systemd ]]
  mkdir -p /etc/systemd/resolved.conf.d
  [[ ! -L /etc/systemd/resolved.conf.d ]]
  mount -t tmpfs -o mode=0755 nhop-dns-config-test /etc/systemd/resolved.conf.d
  cat > /etc/systemd/resolved.conf.d/99-nhop-test.conf <<'CONFIG'
[Resolve]
DNS=
FallbackDNS=
DNSStubListener=yes
DNSSEC=no
DNSOverTLS=no
LLMNR=no
MulticastDNS=no
CONFIG
}
