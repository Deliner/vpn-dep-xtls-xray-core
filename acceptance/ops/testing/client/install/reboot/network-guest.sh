#!/usr/bin/env bash
# Switch only the dedicated reboot client's underlay; preserve the cloud-init input.
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "$(id -u)" == 0 && "$(hostname)" == lima-boot-client ]] || exit 2
state=/opt/nhop-reboot-network
override=/etc/netplan/99-nhop-reboot-manager.yaml
units=(systemd-networkd.service systemd-networkd.socket systemd-networkd-wait-online.service)
[[ ! -e /var/lib/nhop-client/boot.json && ! -e /etc/systemd/system/nhop-client.service ]] || {
  printf 'Remove the owned client fixture before changing its underlay.\n' >&2; exit 2;
}

owned_state() {
  [[ -d "$state" && ! -L "$state" && "$(stat -c '%a:%u:%g' "$state")" == 700:0:0 ]] || return 2
  [[ -f "$state/fixture.sh" && ! -L "$state/fixture.sh" && "$(stat -c '%a:%u:%g' "$state/fixture.sh")" == 700:0:0 ]] || return 2
  (cd "$state"; sha256sum --check inputs.sha256)
  sha256sum --check "$state/cloud-init.sha256"
}

prepare() {
  [[ ! -e "$state" && ! -L "$state" && ! -e "$override" && ! -L "$override" ]] || return 2
  [[ "$(find /etc/netplan -mindepth 1 -maxdepth 1 | wc -l)" == 1 && ! -L /etc/netplan/50-cloud-init.yaml ]] || return 2
  [[ "$(systemctl is-active systemd-networkd.service)" == active ]] || return 2
  for unit in "${units[@]}"; do [[ "$(systemctl is-enabled "$unit")" == enabled ]] || return 2; done
  [[ "$(systemctl show --property=LoadState --value nhop-reboot-network-restore.timer)" == not-found ]] || return 2
  command -v nmcli >/dev/null
  install -d -m 0700 "$state"
  install -m 0700 "$0" "$state/fixture.sh"
  sha256sum /etc/netplan/50-cloud-init.yaml > "$state/cloud-init.sha256"
  netplan get > "$state/baseline.yaml"
  printf 'network:\n  version: 2\n  renderer: NetworkManager\n  ethernets:\n    eth0:\n      networkmanager:\n        passthrough:\n          ipv4.dhcp-client-id: mac\n' > "$state/override.yaml"
  (cd "$state"; sha256sum fixture.sh cloud-init.sha256 baseline.yaml override.yaml) > "$state/inputs.sha256"
  printf 'prepared\n' > "$state/status"
  systemd-run --unit=nhop-reboot-network-restore --on-active=120s "$state/fixture.sh" restore
  systemd-run --unit=nhop-reboot-network-switch --no-block "$state/fixture.sh" switch
}

switch_manager() {
  owned_state
  [[ "$(cat "$state/status")" == prepared && ! -e "$override" && ! -L "$override" ]] || return 2
  install -m 0600 "$state/override.yaml" "$override"
  netplan generate
  systemctl enable NetworkManager.service
  netplan apply
  # Leave units inspectable: the client validates protection of both managers.
  # Netplan must finish reloading networkd before its activation is disabled.
  systemctl disable --now "${units[@]}"
  systemctl start NetworkManager.service
  printf 'switched\n' > "$state/status"
}

confirm() {
  owned_state
  [[ "$(cat "$state/status")" == switched ]] || return 2
  [[ "$(systemctl is-active NetworkManager.service)" == active ]] || return 2
  [[ "$(systemctl is-active systemd-networkd.service)" == inactive ]] || return 2
  [[ "$(nmcli --get-values GENERAL.STATE device show eth0)" == '100 '* ]] || return 2
  cmp "$state/override.yaml" "$override"
  systemctl stop nhop-reboot-network-restore.timer
  nmcli --terse --fields GENERAL,IP4,DHCP4 device show eth0 > "$state/lease.txt"
  systemctl show --property=Id,LoadState,ActiveState,UnitFileState "${units[@]}" NetworkManager.service > "$state/selected-units.txt"
  printf 'confirmed\n' > "$state/status"
}

restore() {
  owned_state
  if [[ -e "$override" || -L "$override" ]]; then
    [[ -f "$override" && ! -L "$override" ]] || return 2
    cmp "$state/override.yaml" "$override"
    rm -- "$override"
  fi
  systemctl disable --now NetworkManager.service
  systemctl unmask "${units[@]}"
  systemctl enable "${units[@]}"
  netplan generate
  systemctl daemon-reload
  netplan apply
  systemctl start systemd-networkd.service
  netplan get > "$state/restored.yaml"
  cmp "$state/baseline.yaml" "$state/restored.yaml"
  printf 'restored\n' > "$state/status"
}

case "$1" in
  prepare) prepare ;;
  switch) switch_manager ;;
  confirm) confirm ;;
  restore) restore ;;
  *) exit 2 ;;
esac
