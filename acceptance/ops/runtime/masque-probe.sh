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
if ! CGO_ENABLED=1 go -C "$base/source" test -race -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  github.com/apernet/quic-go -run '^(TestDatagramQueue|TestNHOPReceiveQueue)' -count=1 -v > "$output/receive-control.log" 2>&1; then
  cat "$output/receive-control.log" >&2
  exit 1
fi
receive_drops=2
if grep -q '^NHOP_QUIC_CONFIGURED_POLICY packets=2 bytes=6 drops=2 preserved=true$' "$output/receive-control.log"; then
  receive_drops=4
  grep -Eq '^NHOP_QUIC_QUEUE_DROP .*buffered=6 budget=6 queued=2 limit=2 bytes=1 prefix=67$' "$output/receive-control.log"
  grep -Eq '^NHOP_QUIC_QUEUE_DROP .*buffered=3 budget=6 queued=1 limit=2 bytes=7 prefix=746f6f6c6f6e67$' "$output/receive-control.log"
fi
[[ "$(grep -c '^NHOP_QUIC_QUEUE_DROP ' "$output/receive-control.log")" == "$receive_drops" ]]
grep -Eq '^NHOP_QUIC_QUEUE_DROP .*queued=128 limit=128 bytes=128 prefix=(ab){96}$' "$output/receive-control.log"
grep -Eq '^NHOP_QUIC_QUEUE_DROP .*queued=128 limit=128 bytes=1 prefix=ff$' "$output/receive-control.log"
grep -q '^NHOP_QUIC_RECEIVE_POLICY limit=128 drops=2 fifo=true copy=true recovery=true close=true$' "$output/receive-control.log"
if ! CGO_ENABLED=1 go -C "$base/source" test -race -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  github.com/apernet/quic-go -run '^(TestSendQueue|TestNHOPSendQueueSizeDrop|TestNHOPSendQueuePMTU)' -count=1 -v > "$output/send-control.log" 2>&1; then
  cat "$output/send-control.log" >&2
  exit 1
fi
if grep -q '^NHOP_SEND_POLICY ' "$output/send-control.log"; then
  for kernel in true false; do
    for probe in true false; do
      terminal=true
      [[ "$probe" != true ]] || terminal=false
      grep -Eq "NHOP_QUIC_SEND_SIZE_DROP .*bytes=1300 gso=0 kernel=$kernel probe=$probe terminal=$terminal " "$output/send-control.log"
      grep -q "^NHOP_SEND_POLICY probe=$probe terminal=$terminal preserved=true$" "$output/send-control.log"
    done
  done
else
  for kernel in true false; do
    grep -Eq "NHOP_QUIC_SEND_SIZE_DROP .*bytes=1300 gso=1300 kernel=$kernel " "$output/send-control.log" || {
      printf 'Diagnostic control did not observe send-size failure (kernel=%s).\n' "$kernel" >&2
      exit 1
    }
  done
fi
if grep -q '"send_queue_policy": "adaptive-pmtu"' "$output/probe.json"; then
  for kernel in true false; do
    grep -Eq "NHOP_QUIC_SEND_SIZE_DROP .*bytes=1300 gso=0 kernel=$kernel probe=false terminal=false failed_size=1300 " "$output/send-control.log"
  done
  grep -q '^NHOP_ADAPTIVE_SEND_POLICY size=1300 terminal=false preserved=true$' "$output/send-control.log"
  grep -Eq 'NHOP_QUIC_SEND_SIZE_DROP .*bytes=1200 gso=0 kernel=true probe=false terminal=true failed_size=1200 ' "$output/send-control.log"
  grep -q '^NHOP_ADAPTIVE_SEND_POLICY size=1200 terminal=true preserved=true$' "$output/send-control.log"
fi
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
  for source in '192\.0\.2\.1:' '\[fd00::1\]:'; do
    grep -Eq "NHOP_INNER_UDP .*phase=$phase .*src=$source.*payload=52 digest=[a-f0-9]{64}$" "$output/packet-control.log"
  done
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
for phase in offer read; do
  for source in '192\.0\.2\.1:' '\[fd00::1\]:'; do
    grep -Eq "NHOP_DEVICE_UDP .*phase=$phase queued=0 .*src=$source.*payload=52 digest=[a-f0-9]{64}$" "$output/device-control.log"
  done
done
if ! CGO_ENABLED=1 go -C "$base/source" test -race -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  ./proxy/masque -count=1 -v > "$output/server-control.log" 2>&1; then
  cat "$output/server-control.log" >&2
  exit 1
fi
grep -q '^NHOP_SERVER_QUEUE_POLICY limit=512 full=reject closed=reject fifo=true ownership=true$' "$output/server-control.log"
for phase in offer admitted dequeue write_done; do
  grep -Eq "NHOP_SERVER_QUEUE .*phase=$phase .*limit=512 .*network=tcp " "$output/server-control.log"
done
for phase in drop_full drop_closed; do
  grep -Eq "NHOP_SERVER_QUEUE .*phase=$phase queued=512 limit=512 .*network=udp .*digest=[a-f0-9]{64}$" "$output/server-control.log"
done
for failed in true false; do
  grep -Eq "NHOP_SERVER_QUEUE .*phase=write_done .*limit=512 failed=$failed .*network=udp .*digest=[a-f0-9]{64}$" "$output/server-control.log"
done
for kind in none packet-too-big closed canceled timeout other; do
  grep -Eq "NHOP_SERVER_QUEUE .*phase=write_done .*write_error=$kind .*network=udp " "$output/server-control.log"
done
grep -q '^--- PASS: TestNHOPServerQueueBindingPacketTooBig ' "$output/server-control.log"
mkdir "$output/bin"
GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go -C "$base/source" build \
  -trimpath -buildvcs=false -modfile "$output/probe.mod" -overlay "$output/overlay.json" \
  -ldflags '-s -w -buildid= -X github.com/xtls/xray-core/core.build=nhop-diagnostic-masque-9' \
  -o "$output/bin/xray" ./main
go -C "$base/source" mod verify
candidate_inventory "$base" > "$output/input.after.sha256"
cmp "$output/input.before.sha256" "$output/input.after.sha256"
(cd "$output/dependency"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/dependency.after.sha256"
cmp "$output/dependency.before.sha256" "$output/dependency.after.sha256"
{
  printf 'kind=diagnostic-only\nprobe_label=nhop-diagnostic-masque-9\nbase_candidate=%s\ntarget=%s\n' "$base" "$target"
  cat "$base/provenance.txt"
  go version -m "$output/bin/xray"
  shasum -a 256 "$root/ops/runtime/masque-probe.sh" "$root/cmd/masqueprobe/main.go" "$root"/internal/quality/runtimebuild/masqueprobe/*.go
  shasum -a 256 "$root"/internal/quality/runtimebuild/masqueprobe/overlay/*.go "$root"/internal/quality/runtimebuild/masqueprobe/packets/*.go
  shasum -a 256 "$root"/internal/quality/runtimebuild/masqueprobe/packettrace/*.go
} > "$output/provenance.txt"
(cd "$output"; shasum -a 256 bin/xray overlay/* overlay.json probe.json probe.mod probe.sum provenance.txt control.log receive-control.log send-control.log packet-control.log device-control.log server-control.log input.*.sha256 dependency.*.sha256) > "$output/SHA256SUMS"
printf 'diagnostic-built\n' > "$output/status"
printf 'Diagnostic only, logs queues, TCP headers and UDP digests: %s/bin/xray\nUse only with synthetic lab traffic.\n' "$output"
