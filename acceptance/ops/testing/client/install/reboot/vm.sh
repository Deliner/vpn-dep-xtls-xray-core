#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
reboot_lab_environment
reboot_lab_owned
case "${1:-}" in
  start)
    for node in "${reboot_nodes[@]}"; do
      "$lab_lima" start --tty=false --timeout=5m "$node"
      [[ "$(reboot_lab_shell "$node" hostname)" == "lima-$node" ]] || exit 2
    done
    ;;
  stop)
    for node in "${reboot_nodes[@]}"; do "$lab_lima" stop "$node"; done
    ;;
  status) "$lab_lima" list "${reboot_nodes[@]}" ;;
  shell) shift; reboot_lab_shell "$@" ;;
  *) printf 'Use just client-reboot-lab-start/stop/status/shell.\n' >&2; exit 2 ;;
esac
