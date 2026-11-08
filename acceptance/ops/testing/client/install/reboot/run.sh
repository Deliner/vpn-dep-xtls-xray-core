#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../../../../runtime/integrity.sh"
reboot_lab_environment
reboot_lab_owned
run="$(mktemp -d "$reboot_data/runs/run-XXXXXXXX")"
printf 'preparing\n' > "$run/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
mkdir "$run/bin"
for directory in "$1" "$2"; do
  candidate_complete "$directory"
  [[ "$(sed -n 's/^target=//p' "$directory/provenance.txt")" == "linux/$lab_arch" ]] || exit 2
  candidate_inventory "$directory" > "$run/observed.sha256"
  cmp "$run/observed.sha256" "$directory/SHA256SUMS"
done
rm "$run/observed.sha256"
cp "$1/bin/xray" "$2/bin/sing-box" "$2/bin/libcronet.so" "$run/bin/"
cp "$1/provenance.txt" "$run/xray-provenance.txt"
cp "$2/provenance.txt" "$run/sing-box-provenance.txt"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false -o "$run/reboot.test" ./internal/quality/linuxlab/reboot
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$run/bin/nhop-client" ./cmd/nhop-client
{
  go version
  git -C "$lab_root" rev-parse HEAD
  git -C "$lab_root" status --porcelain
  (cd "$lab_root"; find internal -name '*.go' -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256; shasum -a 256 go.mod go.sum cmd/nhop-client/main.go; find ops/testing/client -name '*.sh' -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256)
} > "$run/provenance.txt"
(cd "$run"; shasum -a 256 reboot.test bin/* *provenance.txt) > "$run/inputs.sha256"
for node in "${reboot_nodes[@]}"; do
  reboot_lab_shell "$node" sudo -n apt-get update > "$run/$node-prerequisites.log" 2>&1
  reboot_lab_shell "$node" sudo -n apt-get install -y nftables iproute2 dbus systemd-resolved ca-certificates curl >> "$run/$node-prerequisites.log" 2>&1
done
export NHOP_REBOOT_ROOT="$run" NHOP_REBOOT_REPOSITORY="$lab_root"
printf 'running\n' > "$run/status"
if go test -race -count=1 -v -timeout=20m ./internal/quality/linuxlab/reboot -run '^TestRebootAcceptance$' > "$run/host.log" 2>&1; then
  printf 'passed\n' > "$run/status"
  result=0
else
  result=$?
fi
cat "$run/host.log"
printf 'Real Linux reboot evidence: %s\n' "$run"
exit "$result"
