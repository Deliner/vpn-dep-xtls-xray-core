#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
source "$root/ops/testing/linux/package-common.sh"
[[ $# == 2 || $# == 4 ]] || exit 2
package_begin "$1" "$2"
previous="${3:-}" previous_sha="${4:-}"
if [[ -n "$previous" || -n "$previous_sha" ]]; then
  [[ -f "$previous" && ! -L "$previous" && "$previous_sha" =~ ^[0-9a-f]{64}$ ]] || { printf 'Previous installer requires a regular file and explicit SHA-256.\n' >&2; exit 2; }
  [[ "$(shasum -a 256 "$previous" | awk '{print $1}')" == "$previous_sha" ]] || { printf 'Previous installer SHA-256 mismatch.\n' >&2; exit 1; }
  header="$(od -An -v -tx1 -N20 "$previous" | tr -d '[:space:]')"
  machine=3e00
  [[ "$arch" != arm64 ]] || machine=b700
  [[ "${header:0:12}" == 7f454c460201 && "${header:36:4}" == "$machine" ]] || { printf 'Previous installer must be a Linux ELF for %s.\n' "$arch" >&2; exit 2; }
  cp "$previous" "$bundle/bin/previous-nhop-node"
  chmod 0755 "$bundle/bin/previous-nhop-node"
  [[ "$(shasum -a 256 "$bundle/bin/previous-nhop-node" | awk '{print $1}')" == "$previous_sha" ]] || exit 1
fi
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go build -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$bundle/bin/nhop-node" "$root/cmd/nhop-node"
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go test -c -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$bundle/installer.test" "$root/internal/node/installer"
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go test -c -trimpath -buildvcs=false \
  -ldflags=-buildid= -o "$bundle/node.test" "$root/cmd/nhop-node"
package_test
cp "$root/ops/testing/linux/installer/guest.sh" "$bundle/guest.sh"
package_seal installer
