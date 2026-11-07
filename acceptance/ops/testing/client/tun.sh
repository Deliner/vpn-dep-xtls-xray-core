#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# -ge 2 && $# -le 3 ]] || exit 2
case "${3:-tunnel}" in tunnel|block) ;; *) exit 2 ;; esac
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
client_lab_begin
client_lab_candidates "$1" "$2"
client_lab_run tun "${3:-tunnel}"
