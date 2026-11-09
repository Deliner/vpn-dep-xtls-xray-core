#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 4 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
recipe="$root/ops/runtime/xray/quic"
source "$root/ops/runtime/integrity.sh"
source "$recipe/inputs.sh"
base="$(cd "$1" && pwd -P)"
case "$base/" in "$root/build/runtime-candidates/"*) ;; *) exit 2 ;; esac
case "$2" in build/quic-mtu-regression/*) ;; *) exit 2 ;; esac
case "$3" in control|patched) variant="$3" ;; *) exit 2 ;; esac
case "$4" in build/runtime-candidates/*) ;; *) exit 2 ;; esac
name="${4#build/runtime-candidates/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/runtime-candidates" ]]
[[ "$(go env GOVERSION)" == go1.27.3 ]]
export GOWORK=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly
export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOPRIVATE= GONOPROXY= GONOSUMDB=
mkdir "$root/$4"
work="$root/$4"
printf 'building\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
candidate_complete "$base"
candidate_inventory "$base" > "$work/base.inventory.sha256"
cmp "$base/SHA256SUMS" "$work/base.inventory.sha256"
[[ "$(sed -n 's/^engine=//p' "$base/provenance.txt")" == xray ]]
target="$(sed -n 's/^target=//p' "$base/provenance.txt")"
case "$target" in linux/amd64|linux/arm64) ;; *) exit 2 ;; esac
cmp "$base/source/go.mod" "$recipe/../go.mod"
cmp "$base/source/go.sum" "$recipe/../go.sum"
quic_regression_verify "$root" "$root/$2" "$work/regression.inventory.sha256"
cp -R "$base/source" "$work/source"
mkdir "$work/source/nhop-dependencies" "$work/source/nhop-dependencies/apernet-quic-go" "$work/bin"
dependency="$work/source/nhop-dependencies/apernet-quic-go"
COPYFILE_DISABLE=1 tar --no-xattrs --exclude=.git -cf - -C "$root/$2/$variant" . | tar -xf - -C "$dependency"
go -C "$work/source" mod edit -replace github.com/apernet/quic-go=./nhop-dependencies/apernet-quic-go
CGO_ENABLED=1 go -C "$work/source" test -race -count=1 ./transport/internet/hysteria/congestion/... > "$work/consumer-tests.log" 2>&1
label=nhop-quic-mtu-experiment-1
GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go -C "$work/source" build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X github.com/xtls/xray-core/core.build=$label" -o "$work/bin/xray" ./main
go -C "$work/source" mod verify > "$work/module-verify.log"
candidate_inventory "$base" > "$work/base.after.sha256"
cmp "$work/base.inventory.sha256" "$work/base.after.sha256"
quic_regression_verify "$root" "$root/$2" "$work/regression.after.sha256"
candidate_inventory "$work" > "$work/SHA256SUMS"
{
  printf 'engine=xray\nbuild_label=%s\ntarget=%s\nexperiment_only=true\nvariant=%s\n' "$label" "$target" "$variant"
  printf 'base_candidate=%s\nregression=%s\nbuild_cgo=0\nbuild_tags=\n' "$1" "$2"
  grep -E '^(base_version|source_url|source_archive_sha256)=' "$base/provenance.txt"
  go version
  go version -m "$work/bin/xray"
  shasum -a 256 "$recipe/"*.sh "$recipe/"*.patch "$base/SHA256SUMS" "$base/provenance.txt" "$root/$2/evidence.sha256"
} > "$work/provenance.txt"
printf 'built\n' > "$work/status"
(cd "$work"; find . -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
printf 'Experimental Xray QUIC variant: %s\n' "$work"
