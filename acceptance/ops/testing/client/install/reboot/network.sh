#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && ( "$1" == prepare || "$1" == restore ) ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
reboot_lab_environment
reboot_lab_owned
state=/opt/nhop-reboot-network
# A renderer change can leave Lima's old multiplexed TCP connection half-open.
# Fresh, bounded sessions retain its pinned host key and private SSH identity.
network_shell() {
  ssh -F "$LIMA_HOME/boot-client/ssh.config" -o ControlMaster=no -o ControlPath=none \
    -o ConnectTimeout=5 -o ServerAliveInterval=5 -o ServerAliveCountMax=1 lima-boot-client "$@"
}
[[ "$(network_shell hostname)" == lima-boot-client ]] || exit 2
mkdir -p "$reboot_data/networks"
report="$(mktemp -d "$reboot_data/networks/run-XXXXXXXX")"
if [[ "$1" == prepare ]]; then
  reboot_lab_shell boot-client sudo -n apt-get update > "$report/packages.log" 2>&1
  reboot_lab_shell boot-client sudo -n apt-get install -y --no-install-recommends network-manager >> "$report/packages.log" 2>&1
  temporary="$(reboot_lab_shell boot-client mktemp /home/nhop/nhop-reboot-network-XXXXXXXX)"
  [[ "$temporary" == /home/nhop/nhop-reboot-network-* ]] || exit 2
  source_file="$lab_root/ops/testing/client/install/reboot/network-guest.sh"
  "$lab_lima" copy "$source_file" "boot-client:$temporary"
  expected="$(shasum -a 256 "$source_file" | awk '{print $1}')"
  [[ "$(reboot_lab_shell boot-client sha256sum "$temporary" | awk '{print $1}')" == "$expected" ]] || exit 2
  reboot_lab_shell boot-client sudo -n bash "$temporary" prepare > "$report/prepare.log" 2>&1
  confirmed=false
  for ((attempt = 0; attempt < 30; attempt++)); do
    if network_shell sudo -n "$state/fixture.sh" confirm > "$report/confirm.log" 2>&1; then confirmed=true; break; fi
    if [[ "$(network_shell sudo -n cat "$state/status" 2>/dev/null)" == restored ]]; then break; fi
    sleep 2
  done
  [[ "$confirmed" == true ]] || { printf 'Manager switch not confirmed; local timer restores networkd. Evidence: %s\n' "$report" >&2; exit 1; }
  network_shell rm -- "$temporary"
else
  if [[ "$(network_shell sudo -n cat "$state/status")" != restored ]]; then
    network_shell sudo -n systemd-run --unit=nhop-reboot-network-return --no-block "$state/fixture.sh" restore > "$report/restore-request.log" 2>&1
  fi
  restored=false
  for ((attempt = 0; attempt < 30; attempt++)); do
    if [[ "$(network_shell sudo -n cat "$state/status" 2>/dev/null)" == restored ]]; then restored=true; break; fi
    sleep 2
  done
  [[ "$restored" == true ]] || { printf 'Underlay recovery pending; retained VM and state.\n' >&2; exit 1; }
fi
network_shell sudo -n tar -czf - -C "$state" . > "$report/guest.tar.gz"
network_shell sudo -n systemctl show --property=Id,LoadState,ActiveState,UnitFileState systemd-networkd.service NetworkManager.service > "$report/units.txt"
network_shell sudo -n journalctl --no-pager -u nhop-reboot-network-switch.service -u nhop-reboot-network-restore.service -u nhop-reboot-network-return.service > "$report/journal.log"
if [[ "$1" == restore ]]; then
  if [[ "$(network_shell systemctl show --property=LoadState --value nhop-reboot-network-restore.timer)" != not-found ]]; then
    network_shell sudo -n systemctl stop nhop-reboot-network-restore.timer
  fi
  for unit in nhop-reboot-network-switch.service nhop-reboot-network-restore.service nhop-reboot-network-return.service; do
    if [[ "$(network_shell systemctl show --property=ActiveState --value "$unit")" == failed ]]; then network_shell sudo -n systemctl reset-failed "$unit"; fi
  done
  network_shell sudo -n rm -rf -- "$state"
fi
if [[ -S "$LIMA_HOME/boot-client/ssh.sock" ]]; then ssh -F "$LIMA_HOME/boot-client/ssh.config" -O exit lima-boot-client; fi
printf 'Completed owned reboot network %s; evidence: %s\n' "$1" "$report"
