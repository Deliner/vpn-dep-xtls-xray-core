#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
client_lab_begin ./internal/control/client/session/acceptance
client_lab_candidates "$1" "$2"
client_lab_run session
