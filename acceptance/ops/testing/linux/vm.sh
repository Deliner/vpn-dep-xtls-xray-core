#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
lab_environment
lab_owned
case "${1:-}" in
  start)
    [[ -L "$LIMA_HOME/$lab_instance/disk" && "$(readlink "$LIMA_HOME/$lab_instance/disk")" == "$lab_data/disk" ]] || { printf 'Run just lab-setup to prepare the private disk.\n' >&2; exit 2; }
    "$lab_lima" start --tty=false --timeout=5m "$lab_instance"
    lab_shell sh -c 'uname -a; command -v curl; test "$(uname -s)" = Linux'
    ;;
  stop) "$lab_lima" stop "$lab_instance" ;;
  status) "$lab_lima" list "$lab_instance" ;;
  shell) shift; lab_shell "$@" ;;
  *) printf 'Use just lab-start, lab-stop, lab-status or lab-shell.\n' >&2; exit 2 ;;
esac
