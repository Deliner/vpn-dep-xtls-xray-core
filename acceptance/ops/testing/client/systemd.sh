#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
client_lab_begin ./internal/control/client/acceptance
client_lab_candidates "$1" "$2"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$run/bin/nhop-client" ./cmd/nhop-client
cp "$lab_root/ops/testing/client/systemd-guest.sh" "$run/systemd-guest.sh"
client_lab_run systemd "$3"
