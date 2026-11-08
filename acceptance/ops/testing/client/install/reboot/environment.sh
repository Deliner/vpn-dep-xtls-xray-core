#!/usr/bin/env bash
# A separate client and exit share only this checkout-owned userspace network.
source "$(dirname "${BASH_SOURCE[0]}")/../../../linux/environment.sh"

reboot_lab_environment() {
  lab_environment
  reboot_data="$lab_root/build/client-reboot-lab"
  export LIMA_HOME="$HOME/.ssh-custom/nhop-clientboot-$lab_id"
  reboot_nodes=(boot-client boot-exit)
}

reboot_lab_owned() {
  lab_owned
  [[ -d "$reboot_data" && ! -L "$reboot_data" ]] || return 2
  for node in "${reboot_nodes[@]}"; do
    [[ -L "$LIMA_HOME/$node/disk" && "$(readlink "$LIMA_HOME/$node/disk")" == "$reboot_data/disks/$node" ]] || {
      printf 'Run just client-reboot-lab-setup to prepare the owned disks.\n' >&2; return 2;
    }
  done
}

reboot_lab_shell() {
  local node="$1"
  shift
  case "$node" in boot-client|boot-exit) ;; *) printf 'Unknown client reboot lab node.\n' >&2; return 2 ;; esac
  "$lab_lima" shell --workdir /home/nhop "$node" "$@"
}
