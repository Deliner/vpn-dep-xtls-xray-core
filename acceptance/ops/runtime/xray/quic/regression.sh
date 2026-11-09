#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
recipe="$root/ops/runtime/xray/quic"
case "${1:-}" in build/quic-mtu-regression/*) ;; *) printf 'Use a new build/quic-mtu-regression/ directory.\n' >&2; exit 2 ;; esac
name="${1#build/quic-mtu-regression/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/quic-mtu-regression" ]] || exit 2
mkdir -p "$root/build/quic-mtu-regression"
mkdir "$root/$1"
work="$root/$1"
printf 'running\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GOPRIVATE= GONOPROXY= GONOSUMDB= GOTOOLCHAIN=local GOFLAGS=-mod=readonly
[[ "$(go env GOVERSION)" == go1.27.3 ]] || exit 2
mkdir "$work/graph"
cp "$recipe/../go.mod" "$recipe/../go.sum" "$work/graph/"
module=github.com/Deliner/vpn-dep-apernet-quic-go
version=v0.61.1-0.20261109033109-84023edbb399
[[ "$(go -C "$work/graph" list -m -f '{{.Replace.Path}}@{{.Replace.Version}}' github.com/apernet/quic-go)" == "$module@$version" ]] || exit 2
go -C "$work/graph" mod download -json "$module@$version" > "$work/module.json"
grep -Fq '"Sum": "h1:gvzU9Neeo6OJm396ji4Og8XI7QpsO+iyuIWfHxZdWLk="' "$work/module.json"
go -C "$work/graph" mod verify > "$work/module-verify.log"
source="$(go -C "$work/graph" list -m -f '{{.Replace.Dir}}' github.com/apernet/quic-go)"
[[ -d "$source" && ! -L "$source" ]] || exit 2
for variant in control patched; do
  cp -R "$source" "$work/$variant"
  chmod -R u+w "$work/$variant"
  git -C "$work/$variant" init --quiet
  git -C "$work/$variant" apply --check --whitespace=error-all "$recipe/mtu-probe-tests.patch"
  git -C "$work/$variant" apply --whitespace=error-all "$recipe/mtu-probe-tests.patch"
done
git -C "$work/patched" apply --check --whitespace=error-all "$recipe/mtu-probe-loss.patch"
git -C "$work/patched" apply --whitespace=error-all "$recipe/mtu-probe-loss.patch"
{
  go version
  git -C "$root" rev-parse HEAD
  shasum -a 256 "$recipe/"*.patch "$recipe/regression.sh" "$recipe/../go.mod" "$recipe/../go.sum"
  go -C "$work/patched" list -m all
} > "$work/provenance.txt"
set +e
CGO_ENABLED=1 go -C "$work/control" test -race -count=1 -timeout=60s -v -run '^TestMTUProbe' ./internal/ackhandler > "$work/control.log" 2>&1
control_result=$?
set -e
printf '%s\n' "$control_result" > "$work/control.exit"
[[ "$control_result" == 1 ]]
grep -Fq 'PMTU probe loss leaked into extended congestion control' "$work/control.log"
grep -Fq 'PMTU-only loss triggered extended congestion control' "$work/control.log"
grep -Fq -- '--- PASS: TestMTUProbeACKStillContributesToExtendedDelivery' "$work/control.log"
! grep -Eq 'build failed|DATA RACE|panic:' "$work/control.log"
CGO_ENABLED=1 go -C "$work/patched" test -race -count=20 -timeout=60s -v -run '^TestMTUProbe' ./internal/ackhandler > "$work/patched.log" 2>&1
CGO_ENABLED=1 go -C "$work/patched" test -race -count=1 -timeout=120s ./internal/ackhandler ./internal/congestion > "$work/packages.log" 2>&1
(cd "$work"; find graph control patched -type d -name .git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/sources.sha256"
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type d -name .git -prune -o -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
printf 'QUIC PMTU probe regression: %s\n' "$work"
