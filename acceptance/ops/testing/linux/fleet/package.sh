#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
source "$root/ops/testing/linux/package-common.sh"
[[ $# == 2 ]] || exit 2
package_begin "$1" "$2"
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go build -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$bundle/bin/nhop-fleet" "$root/cmd/nhop-fleet"
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go test -c -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$bundle/fleet.test" "$root/internal/control/fleet/acceptance"
cp "$root/ops/testing/linux/fleet/guest.sh" "$bundle/guest.sh"
package_seal fleet
