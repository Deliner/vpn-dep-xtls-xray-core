#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
source "$root/ops/runtime/integrity.sh"
source "$root/ops/testing/linux/package-common.sh"
[[ $# == 4 ]] || exit 2
package_begin "$1" "$4"

copy_candidate() {
  local engine="$1" path="$2" actual
  path="$(cd "$path" && pwd -P)"
  candidate_complete "$path"
  [[ "$(sed -n 's/^engine=//p' "$path/provenance.txt")" == "$engine" ]] || return 2
  [[ "$(sed -n 's/^target=//p' "$path/provenance.txt")" == "linux/$arch" ]] || { printf 'Wrong candidate target: %s\n' "$engine" >&2; return 2; }
  actual="$output/$engine.actual.sha256"
  candidate_inventory "$path" > "$actual"
  cmp "$path/SHA256SUMS" "$actual" || { printf 'Candidate changed after build: %s\n' "$engine" >&2; return 1; }
  mkdir "$bundle/bin/$engine"
  cp "$path/bin/"* "$bundle/bin/$engine/"
  cp "$path/provenance.txt" "$bundle/bin/$engine/provenance.txt"
  (cd "$bundle/bin/$engine"; sed -n 's|  bin/|  |p' "$actual" | shasum -a 256 --check)
}

copy_candidate xray "$2"
copy_candidate sing-box "$3"
package_test
for probe_package in probe probe/ipcheck; do
  GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go test -c -trimpath -buildvcs=false \
    -ldflags=-buildid= -o "$bundle/${probe_package##*/}.test" "$root/internal/control/$probe_package"
done
cp "$root/ops/testing/linux/guest.sh" "$bundle/guest.sh"
package_seal payload
