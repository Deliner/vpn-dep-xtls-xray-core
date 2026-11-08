#!/usr/bin/env bash
# An explicitly diagnostic overlay; candidate sources and module cache stay unchanged.
set -euo pipefail
umask 077
[[ $# == 2 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "$root/ops/runtime/integrity.sh"
base="$(cd "$1" && pwd -P)"
case "$base/" in "$root/build/runtime-candidates/"*) ;; *) exit 2 ;; esac
case "$2" in build/runtime-probes/*) ;; *) printf 'Use a fresh build/runtime-probes/ directory.\n' >&2; exit 2 ;; esac
name="${2#build/runtime-probes/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/runtime-probes" ]] || exit 2
[[ "$(go env GOVERSION)" == go1.27.3 ]] || exit 2
mkdir -p "$root/build/runtime-probes"
mkdir "$root/$2"
output="$root/$2"
printf 'building\n' > "$output/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$output/status"; fi' EXIT
candidate_complete "$base"
candidate_inventory "$base" > "$output/input.before.sha256"
cmp "$base/SHA256SUMS" "$output/input.before.sha256"
[[ "$(sed -n 's/^engine=//p' "$base/provenance.txt")" == xray ]] || exit 2
target="$(sed -n 's/^target=//p' "$base/provenance.txt")"
case "$target" in linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;; *) exit 2 ;; esac
export GOWORK=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly
export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOPRIVATE= GONOPROXY= GONOSUMDB=
go -C "$base/source" mod verify
module="$(go -C "$base/source" list -m -f '{{.Dir}}' github.com/apernet/quic-go)"
# Go forbids overlays beneath GOMODCACHE. Use a byte-identical private copy and
# an alternate modfile; neither the verified cache nor candidate is modified.
cp -R "$module" "$output/dependency"
(cd "$module"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/dependency.before.sha256"
(cd "$output/dependency"; shasum -a 256 -c "$output/dependency.before.sha256") > "$output/dependency-copy.log"
cp "$base/source/go.mod" "$output/probe.mod"
cp "$base/source/go.sum" "$output/probe.sum"
go -C "$base/source" mod edit -modfile "$output/probe.mod" -replace "github.com/apernet/quic-go=$output/dependency"
go -C "$root" run ./cmd/masqueprobe "$output/dependency" "$base/source" "$output"
if ! go -C "$base/source" test -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  github.com/apernet/quic-go/http3 -run '^TestDatagramReceiving$' -count=1 -v > "$output/control.log" 2>&1; then
  cat "$output/control.log" >&2
  exit 1
fi
if ! go -C "$base/source" test -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  github.com/apernet/quic-go -run '^(TestSendQueue|TestNHOPSendQueueSizeDrop)' -count=1 -v > "$output/send-control.log" 2>&1; then
  cat "$output/send-control.log" >&2
  exit 1
fi
for kernel in true false; do
  grep -Eq "NHOP_QUIC_SEND_SIZE_DROP .*bytes=1300 gso=1300 kernel=$kernel " "$output/send-control.log" || {
    printf 'Diagnostic control did not observe send-size failure (kernel=%s).\n' "$kernel" >&2
    exit 1
  }
done
if ! grep -Eq 'NHOP_H3_QUEUE_DROP .*queued=32 bytes=1 prefix=20$' "$output/control.log"; then
  printf 'Diagnostic control did not observe the expected queue drop.\n' >&2
  cat "$output/control.log" >&2
  exit 1
fi
if ! CGO_ENABLED=1 go -C "$base/source" test -race -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  ./transport/internet/masque/connectip -count=1 -v > "$output/packet-control.log" 2>&1; then
  cat "$output/packet-control.log" >&2
  exit 1
fi
for phase in submit queued deliver; do
  grep -Eq "NHOP_INNER_TCP .*phase=$phase .*payload=36$" "$output/packet-control.log" || {
    printf 'Diagnostic control did not observe inner TCP phase %s.\n' "$phase" >&2
    exit 1
  }
done
if ! CGO_ENABLED=1 go -C "$base/source" test -race -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  ./proxy/wireguard -run '^(TestNHOP|TestNetTUN)' -count=1 -v > "$output/device-control.log" 2>&1; then
  cat "$output/device-control.log" >&2
  exit 1
fi
for boundary in 'offer queued=0' 'read queued=1'; do
  grep -Eq "NHOP_DEVICE_TCP .*phase=$boundary .*payload=36$" "$output/device-control.log" || {
    printf 'Diagnostic control did not observe device boundary %s.\n' "$boundary" >&2
    exit 1
  }
done
mkdir "$output/bin"
GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go -C "$base/source" build \
  -trimpath -buildvcs=false -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  -ldflags '-s -w -buildid= -X github.com/xtls/xray-core/core.build=nhop-diagnostic-masque-4' \
  -o "$output/bin/xray" ./main
go -C "$base/source" mod verify
candidate_inventory "$base" > "$output/input.after.sha256"
cmp "$output/input.before.sha256" "$output/input.after.sha256"
(cd "$output/dependency"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/dependency.after.sha256"
cmp "$output/dependency.before.sha256" "$output/dependency.after.sha256"
{
  printf 'kind=diagnostic-only\nprobe_label=nhop-diagnostic-masque-4\nbase_candidate=%s\ntarget=%s\n' "$base" "$target"
  cat "$base/provenance.txt"
  go version -m "$output/bin/xray"
  shasum -a 256 "$root/ops/runtime/masque-probe.sh" "$root/cmd/masqueprobe/main.go" "$root"/internal/quality/runtimebuild/masqueprobe/*.go
  shasum -a 256 "$root"/internal/quality/runtimebuild/masqueprobe/packettrace/*.go
} > "$output/provenance.txt"
(cd "$output"; shasum -a 256 bin/xray overlay/* overlay.json probe.json probe.mod probe.sum provenance.txt control.log send-control.log packet-control.log device-control.log input.*.sha256 dependency.*.sha256) > "$output/SHA256SUMS"
printf 'diagnostic-built\n' > "$output/status"
printf 'Diagnostic only, logs dropped prefixes and inner TCP headers: %s/bin/xray\nUse only with synthetic lab traffic.\n' "$output"
