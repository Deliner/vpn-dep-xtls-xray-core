#!/usr/bin/env bash
set -euo pipefail
umask 077
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
source "$root/ops/runtime/integrity.sh"
[[ $# == 2 && "$(go env GOVERSION)" == go1.27.3 ]] || exit 2
candidate="$(cd "$1" && pwd -P)"
case "$candidate/" in "$root/build/runtime-candidates/"*) ;; *) exit 2 ;; esac
case "$2" in build/tuic-regression/*) ;; *) exit 2 ;; esac
name="${2#build/tuic-regression/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/tuic-regression" ]] || exit 2
candidate_complete "$candidate"
mkdir -p "$root/build/tuic-regression"
mkdir "$root/$2"
work="$root/$2"
printf 'running\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
candidate_inventory "$candidate" > "$work/candidate-before.sha256"
cmp "$candidate/SHA256SUMS" "$work/candidate-before.sha256"
export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GOPRIVATE= GONOPROXY= GONOSUMDB= GOTOOLCHAIN=local GOFLAGS=-mod=readonly
module=github.com/sagernet/sing-quic
version=v0.7.2-0.20261020082049-7e0a5869a5f4
[[ "$(go -C "$candidate/source" list -m -f '{{.Version}}' "$module")" == "$version" ]] || exit 2
[[ "$(go -C "$candidate/source" list -m -f '{{.Replace.Dir}}' "$module")" == "$candidate/source/nhop-dependencies/sing-quic" ]] || exit 2
cp -R "$candidate/source/nhop-dependencies/sing-quic" "$work/source"
cp "$candidate/source/go.mod" "$candidate/source/go.sum" "$work/source/"
go -C "$work/source" mod edit -module="$module" -droprequire="$module" "-dropreplace=$module@$version"
git -C "$work/source" init --quiet
patch="$root/ops/runtime/sing-box/tuic/shutdown-tests.patch"
git -C "$work/source" apply --check --whitespace=error-all "$patch"
git -C "$work/source" apply --whitespace=error-all "$patch"
rm -rf "$work/source/.git"
go -C "$work/source" mod verify > "$work/module-verify.log"
{
  uname -a
  go version
  printf 'execution_host=%s/%s\n' "$(go env GOHOSTOS)" "$(go env GOHOSTARCH)"
  cat "$candidate/provenance.txt"
  shasum -a 256 "$patch" "${BASH_SOURCE[0]}"
} > "$work/provenance.txt"
CGO_ENABLED=1 go -C "$work/source" test -race -count=20 -timeout=120s -v -run '^TestServerShutdown$' ./tuic > "$work/patched.log" 2>&1
cat "$work/patched.log"
for early in false true; do
  for cancel in false true; do
    [[ "$(grep -Fc -- "--- PASS: TestServerShutdown/early=$early/cancel=$cancel (" "$work/patched.log")" == 20 ]] || exit 1
  done
done
candidate_inventory "$candidate" > "$work/candidate-after.sha256"
cmp "$work/candidate-before.sha256" "$work/candidate-after.sha256"
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
printf 'Candidate TUIC shutdown verified on %s/%s: %s\n' "$(go env GOHOSTOS)" "$(go env GOHOSTARCH)" "$work"
