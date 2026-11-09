#!/usr/bin/env bash
# Read-only host bounds: NSS, routing, DNS and the absence of our temporary service.
installer_host_baseline() {
  sudo -n sha256sum /etc/passwd /etc/group /etc/shadow /etc/gshadow /etc/resolv.conf || return 1
  ip -4 -j rule || return 1
  ip -6 -j rule || return 1
  ip -4 -j route || return 1
  ip -6 -j route || return 1
}

installer_host_clean() {
  local path output
  for path in /opt/vpn-nhop /etc/systemd/system/vpn-nhop.service /run/systemd/system/vpn-nhop.service.d \
    /etc/systemd/system/multi-user.target.wants/vpn-nhop.service /etc/ufw/applications.d/vpn-nhop; do
    [[ ! -e "$path" && ! -L "$path" ]] || return 2
  done
  if getent passwd vpn-nhop || getent group vpn-nhop; then return 2; fi
  if pgrep -x 'nhop-node|xray|sing-box'; then return 2; fi
  output="$(systemctl show -p LoadState -p ActiveState -p UnitFileState vpn-nhop.service | LC_ALL=C sort)" || return 1
  [[ "$output" == $'ActiveState=inactive\nLoadState=not-found\nUnitFileState=' ]] || return 2
  printf 'service/account/group/paths/processes absent\n'
}
