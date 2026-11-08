#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
client_lab_begin ./internal/control/client/acceptance
client_lab_candidates "$1" "$2"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$run/bin/nhop-client" ./cmd/nhop-client
client_lab_run protocols "$3"
