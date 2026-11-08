#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
fleet_lab_environment
fleet_lab_owned
case "${1:-}" in
  start)
    for node in "${fleet_nodes[@]}"; do
      "$lab_lima" start --tty=false --timeout=5m "$node"
      [[ "$(fleet_lab_shell "$node" hostname)" == "lima-$node" ]] || exit 2
    done
    ;;
  stop)
    for node in "${fleet_nodes[@]}"; do
      "$lab_lima" stop "$node"
    done
    ;;
  status) "$lab_lima" list "${fleet_nodes[@]}" ;;
  shell) shift; fleet_lab_shell "$@" ;;
  *) printf 'Use just fleet-lab-start, fleet-lab-stop, fleet-lab-status or fleet-lab-shell.\n' >&2; exit 2 ;;
esac
