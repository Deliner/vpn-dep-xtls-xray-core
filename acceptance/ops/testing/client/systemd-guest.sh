#!/usr/bin/env bash
# Sourced only by the owned private-namespace runner. PID 1 is the real guest systemd.
client_systemd_mounts() {
  [[ "$(cat /proc/1/comm)" == systemd ]] || return 2
  export NHOP_CLIENT_SYSTEMD_ROOT="/run/nhop-service-test-$$"
  mkdir -m 0711 "$NHOP_CLIENT_SYSTEMD_ROOT"
  mkdir -m 0755 "$NHOP_CLIENT_SYSTEMD_ROOT/"{resolve,netif,session}
  mkdir -m 0711 "$NHOP_CLIENT_SYSTEMD_ROOT/control"
  if [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]]; then client_installed_mounts; fi
  # Stable guest-visible sources let PID 1 bind the same private state without
  # relying on resolution of /proc/PID/root magic links across mount namespaces.
  mount --bind "$NHOP_CLIENT_SYSTEMD_ROOT/resolve" /run/systemd/resolve
  mount --bind "$NHOP_CLIENT_SYSTEMD_ROOT/netif" /run/systemd/netif
  mount --bind "$NHOP_CLIENT_SYSTEMD_ROOT/session" /run/nhop-session-test
}

client_systemd_prepare() {
  [[ "$(cat /proc/1/comm)" == systemd ]] || return 2
  export NHOP_CLIENT_SYSTEMD_UNIT="nhop-client-acceptance-$$.service"
  if [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]]; then export NHOP_CLIENT_SYSTEMD_UNIT=nhop-client.service; fi
  [[ ! -e "/run/systemd/system/$NHOP_CLIENT_SYSTEMD_UNIT" && ! -L "/run/systemd/system/$NHOP_CLIENT_SYSTEMD_UNIT" ]]
  export NHOP_CLIENT_SYSTEMD_BINARY="/opt/nhop-service-test-$$/client binary %\$"
  mkdir -m 0755 "/opt/nhop-service-test-$$"
  cp bin/nhop-client "$NHOP_CLIENT_SYSTEMD_BINARY"
  chmod 0755 "$NHOP_CLIENT_SYSTEMD_BINARY"
  mkdir -m 0755 '/run/nhop-session-test/service inputs %$'
  cp bin/xray bin/sing-box bin/libcronet.so '/run/nhop-session-test/service inputs %$/'
  chmod 0755 '/run/nhop-session-test/service inputs %$/'{xray,sing-box,libcronet.so}
  export NHOP_CLIENT_SYSTEMD_TEST=private-netns
  if [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]]; then export NHOP_CLIENT_BINARY="$NHOP_CLIENT_SYSTEMD_BINARY"; fi
}

client_systemd_remove_fixture() {
  [[ "$NHOP_CLIENT_SYSTEMD_ROOT" == "/run/nhop-service-test-$$" && ! -L "$NHOP_CLIENT_SYSTEMD_ROOT" ]] || return 2
  [[ ! -e "/run/systemd/system/${NHOP_CLIENT_SYSTEMD_UNIT:-}" ]] || return 2
  if mountpoint -q /etc/systemd/resolved.conf.d; then umount /etc/systemd/resolved.conf.d || return 1; fi
  if [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]]; then client_installed_unmounts || return 1; fi
  rm -rf -- "$NHOP_CLIENT_SYSTEMD_ROOT" || return 1
  if [[ -n "${NHOP_CLIENT_SYSTEMD_BINARY:-}" ]]; then
    [[ "$NHOP_CLIENT_SYSTEMD_BINARY" == "/opt/nhop-service-test-$$/client binary %\$" && ! -L "/opt/nhop-service-test-$$" ]] || return 2
    rm -rf -- "/opt/nhop-service-test-$$"
  fi
}

client_systemd_cleanup() {
  if [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]]; then client_installed_cleanup; return $?; fi
  local fragment="/run/systemd/system/$NHOP_CLIENT_SYSTEMD_UNIT"
  if [[ ! -e "$fragment" && ! -L "$fragment" ]]; then return 0; fi
  [[ -f "$fragment" && ! -L "$fragment" && -f systemd-unit.txt && -f systemd-unit.sha256 ]] || return 2
  sha256sum --check systemd-unit.sha256 || return 1
  cmp "$fragment" systemd-unit.txt || return 1
  env -u DBUS_SYSTEM_BUS_ADDRESS systemctl stop "$NHOP_CLIENT_SYSTEMD_UNIT" || return 1
  journalctl --no-pager -u "$NHOP_CLIENT_SYSTEMD_UNIT" || return 1
  rm "$fragment" || return 1
  env -u DBUS_SYSTEM_BUS_ADDRESS systemctl daemon-reload || return 1
}
