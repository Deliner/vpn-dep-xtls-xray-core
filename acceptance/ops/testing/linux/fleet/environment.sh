#!/usr/bin/env bash
# Three separately booted machines on a checkout-owned userspace network.
source "$(dirname "${BASH_SOURCE[0]}")/../environment.sh"
fleet_lab_environment() {
  lab_environment
  fleet_data="$lab_root/build/fleet-lab"
  export LIMA_HOME="$HOME/.ssh-custom/nhop-fleetlab-$lab_id"
  fleet_nodes=(fleet-a fleet-b fleet-c)
}

fleet_lab_owned() {
  lab_owned
  [[ ! -L "$fleet_data" && -d "$fleet_data" ]] || return 2
  for node in "${fleet_nodes[@]}"; do
    [[ -L "$LIMA_HOME/$node/disk" && "$(readlink "$LIMA_HOME/$node/disk")" == "$fleet_data/disks/$node" ]] || {
      printf 'Run just fleet-lab-setup to prepare the owned disks.\n' >&2; return 2;
    }
  done
}

fleet_lab_shell() {
  local node="$1"
  shift
  case "$node" in fleet-a|fleet-b|fleet-c) ;; *) printf 'Unknown Fleet lab node.\n' >&2; return 2 ;; esac
  "$lab_lima" shell --workdir /home/nhop "$node" "$@"
}
