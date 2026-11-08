#!/usr/bin/env bash
# This fixture exposes only the real system-unit directory to the private /etc.
# NSS, CA, DNS, controller state and all data traffic remain isolated.
client_installed_mounts() {
  [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]] || return 2
  local path
  for path in /var/lib/nhop-client /etc/systemd/system/nhop-client.service /etc/systemd/system/nhop-client.service.d \
    /run/systemd/system/nhop-client.service /run/systemd/system/nhop-client.service.d; do
    [[ ! -e "$path" && ! -L "$path" ]] || return 2
  done
  mkdir -m 0700 "$NHOP_CLIENT_SYSTEMD_ROOT/units"
  mount --bind /etc/systemd/system "$NHOP_CLIENT_SYSTEMD_ROOT/units"
}

client_installed_config() {
  [[ "${NHOP_CLIENT_INSTALLED_TEST:-}" == private-netns ]] || return 2
  mount --bind "$NHOP_CLIENT_SYSTEMD_ROOT/units" /etc/systemd/system
}

client_installed_cleanup() {
  local fragment=/etc/systemd/system/nhop-client.service override=/etc/systemd/system/nhop-client.service.d/90-nhop-test.conf
  if [[ ! -e "$fragment" && ! -L "$fragment" ]]; then return 0; fi
  [[ -f "$fragment" && ! -L "$fragment" && -f systemd-unit.txt && -f systemd-unit.sha256 ]] || return 2
  sha256sum --check systemd-unit.sha256 || return 1
  cmp "$fragment" systemd-unit.txt || return 1
  [[ -f "$override" && ! -L "$override" && -f systemd-override.txt ]] || return 2
  cmp "$override" systemd-override.txt || return 1
  env -u DBUS_SYSTEM_BUS_ADDRESS systemctl stop nhop-client.service || return 1
  journalctl --no-pager -u nhop-client.service || return 1
  ip netns exec nhop-client-inner "$NHOP_CLIENT_BINARY" stop || return 1
  rm "$override" || return 1
  rmdir /etc/systemd/system/nhop-client.service.d || return 1
  env -u DBUS_SYSTEM_BUS_ADDRESS systemctl daemon-reload || return 1
  ip netns exec nhop-client-inner "$NHOP_CLIENT_BINARY" service-remove || return 1
  # Interrupted tests retain their verified generation for diagnosis; the wrapper
  # reports failure instead of deleting an incomplete/unverified snapshot tree.
}

client_installed_unmounts() {
  [[ ! -e /var/lib/nhop-client && ! -L /var/lib/nhop-client ]] || return 2
  [[ ! -e /etc/systemd/system/nhop-client.service && ! -L /etc/systemd/system/nhop-client.service ]] || return 2
  [[ ! -e /etc/systemd/system/nhop-client.service.d && ! -L /etc/systemd/system/nhop-client.service.d ]] || return 2
  umount /etc/systemd/system || return 1
  umount "$NHOP_CLIENT_SYSTEMD_ROOT/units" || return 1
}
