#!/usr/bin/env bash
# Sourced only after kernel-guest.sh establishes the private namespaces.
client_private_resolved() {
  mount -t tmpfs -o mode=0755 nhop-resolved-test /run/systemd/resolve
  chown systemd-resolve:systemd-resolve /run/systemd/resolve
  mkdir -p /run/nhop-session-test
  mount -t tmpfs -o mode=0755 nhop-session-test /run/nhop-session-test
  export DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/nhop-session-test/bus
  cat > /run/nhop-session-test/bus.conf <<'CONFIG'
<busconfig><type>system</type><listen>unix:path=/run/nhop-session-test/bus</listen><auth>EXTERNAL</auth><policy context="default"><allow user="*"/><allow own="*"/><allow send_destination="*"/><allow receive_sender="*"/></policy></busconfig>
CONFIG
  ip netns exec nhop-client-inner dbus-daemon --nofork --nopidfile --config-file=/run/nhop-session-test/bus.conf > bus.log 2>&1 &
  for attempt in $(seq 1 50); do [[ ! -S /run/nhop-session-test/bus ]] || break; sleep 0.1; done
  [[ -S /run/nhop-session-test/bus ]]
  ip netns exec nhop-client-inner /usr/lib/systemd/systemd-resolved > resolved.log 2>&1 &
  for attempt in $(seq 1 50); do
    if ip netns exec nhop-client-inner resolvectl dns >/dev/null 2>&1; then return 0; fi
    sleep 0.1
  done
  cat bus.log resolved.log >&2
  return 1
}
