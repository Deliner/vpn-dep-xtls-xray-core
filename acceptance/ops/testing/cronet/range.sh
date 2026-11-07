#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/../linux/environment.sh"
[[ $# == 1 ]] || exit 2
lab_environment
lab_owned
candidate="$(cd "$1" && pwd -P)"
[[ "$(cat "$candidate/status")" == native-built ]] || exit 2
(cd "$candidate"; shasum -a 256 --check SHA256SUMS)
mkdir -p "$lab_data/cronet-range"
run="$(mktemp -d "$lab_data/cronet-range/run-XXXXXXXX")"
printf 'preparing\n' > "$run/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; if [[ -f "$run/guest.log" ]]; then cat "$run/guest.log"; fi; fi' EXIT
wrapper="$lab_root/build/dependencies/checkouts/vpn-dep-sagernet-cronet-go"
[[ "$(git -C "$wrapper" remote get-url origin)" == https://github.com/Deliner/vpn-dep-sagernet-cronet-go.git ]] || exit 2
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 GOFLAGS=-mod=readonly \
  go -C "$wrapper/test" test -c -tags with_purego -trimpath -buildvcs=false -o "$run/cronet.test" .
cp "$candidate/bin/libcronet.so" "$run/libcronet.so"
{
  go version
  git -C "$wrapper" rev-parse HEAD
  git -C "$wrapper" diff --stat
  shasum -a 256 "$wrapper/test/range_response_test.go"
  cat "$candidate/source.tsv"
} > "$run/provenance.txt"
(cd "$run"; shasum -a 256 cronet.test libcronet.so provenance.txt) > "$run/SHA256SUMS"
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$run/package.tar.gz" -C "$run" cronet.test libcronet.so provenance.txt SHA256SUMS
run_name="cronet-range-$(basename "$run")"
lab_shell mkdir -p "runs/$run_name"
"$lab_lima" copy "$run/package.tar.gz" "$lab_instance:/home/nhop/runs/$run_name/package.tar.gz"
digest="$(shasum -a 256 "$run/package.tar.gz" | awk '{print $1}')"
lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$run_name" "$digest" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
[[ "$(hostname)" == lima-payload ]]
cd "runs/$1"
printf '%s  package.tar.gz\n' "$2" | sha256sum --check
tar -xzf package.tar.gz
sha256sum --check SHA256SUMS
NHOP_CRONET_LIBRARY="$PWD/libcronet.so" ./cronet.test -test.run '^TestRangeResponseBoundaries$' -test.v -test.timeout 60s
GUEST
printf 'passed\n' > "$run/status"
cat "$run/guest.log"
printf 'Native range regression evidence: %s\n' "$run"
