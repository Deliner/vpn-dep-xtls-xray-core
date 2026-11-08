#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
recipe="$root/ops/runtime/sing-box/tuic"
case "${1:-}" in build/tuic-regression/*) ;; *) printf 'Output must be a new build/tuic-regression/ directory.\n' >&2; exit 2 ;; esac
name="${1#build/tuic-regression/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/tuic-regression" ]] || exit 2
mkdir -p "$root/build/tuic-regression"
mkdir "$root/$1"
work="$root/$1"
printf 'running\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GOPRIVATE= GONOPROXY= GONOSUMDB= GOTOOLCHAIN=local
[[ "$(go env GOVERSION)" == go1.27.3 ]] || exit 2
mkdir "$work/graph"
cp "$recipe/../go.mod" "$recipe/../go.sum" "$work/graph/"
module=github.com/sagernet/sing-quic
version=v0.7.2-0.20261020082049-7e0a5869a5f4
[[ "$(go -C "$work/graph" list -m -f '{{.Version}}' "$module")" == "$version" ]] || exit 2
go -C "$work/graph" mod download -json "$module" > "$work/module.json"
go -C "$work/graph" mod verify > "$work/module-verify.log"
source="$(go env GOMODCACHE)/$module@$version"
[[ -d "$source" && ! -L "$source" ]] || exit 2
for variant in control patched; do
  cp -R "$source" "$work/$variant"
  chmod -R u+w "$work/$variant"
  git -C "$work/$variant" init --quiet
  cp "$recipe/../go.mod" "$recipe/../go.sum" "$work/$variant/"
  go -C "$work/$variant" mod edit -module="$module" -droprequire="$module"
  git -C "$work/$variant" apply --check --whitespace=error-all "$recipe/shutdown-tests.patch"
  git -C "$work/$variant" apply --whitespace=error-all "$recipe/shutdown-tests.patch"
  [[ -f "$work/$variant/tuic/shutdown_test.go" ]]
done
for patch in server-listener service-shutdown; do
  git -C "$work/patched" apply --check --whitespace=error-all "$recipe/$patch.patch"
  git -C "$work/patched" apply --whitespace=error-all "$recipe/$patch.patch"
done
{
  go version
  git -C "$root" rev-parse HEAD
  shasum -a 256 "$recipe/"*.patch "$recipe/regression.sh" "$recipe/../go.mod" "$recipe/../go.sum"
  go -C "$work/patched" list -m all
} > "$work/provenance.txt"
shasum -a 256 "$recipe/"*.patch "$recipe/regression.sh" "$recipe/../go.mod" "$recipe/../go.sum" > "$work/inputs.sha256"
set +e
CGO_ENABLED=1 go -C "$work/control" test -race -count=1 -timeout=60s -v -run '^TestServerShutdown$/early=false/cancel=false$' ./tuic > "$work/control.log" 2>&1
control_result=$?
set -e
cat "$work/control.log"
[[ "$control_result" == 1 ]]
grep -Fq 'peer did not receive QUIC CONNECTION_CLOSE before UDP socket closed' "$work/control.log"
! grep -Eq 'build failed|DATA RACE|panic:' "$work/control.log"
CGO_ENABLED=1 go -C "$work/patched" test -race -count=20 -timeout=120s -v -run '^TestServerShutdown$' ./tuic > "$work/patched.log" 2>&1
cat "$work/patched.log"
(cd "$work"; find graph control patched -type d -name .git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/sources.sha256"
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type d -name .git -prune -o -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
printf 'TUIC shutdown regression: %s\n' "$work"
