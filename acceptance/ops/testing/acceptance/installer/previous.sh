#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 2 ]] || exit 2
arch="$1"
case "$arch" in amd64|arm64) ;; *) exit 2 ;; esac
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
source "$root/ops/testing/acceptance/installer/source.sh"
[[ ! -e "$2" && ! -L "$2" && -d "$(dirname "$2")" ]] || exit 2
output="$(cd "$(dirname "$2")" && pwd -P)/$(basename "$2")"
case "$output/" in "$root/build/"*) ;; *) exit 2 ;; esac
mkdir -m 0700 "$output" "$output/source" "$output/bin"
printf 'building\n' > "$output/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$output/status"; fi' EXIT
installer_previous_archive "$root" "$output/source.tar"
tar -xf "$output/source.tar" -C "$output/source"
(cd "$output/source"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/source.sha256"
(
  cd "$output/source"
  CGO_ENABLED=0 GOWORK=off GOOS=linux GOARCH="$arch" go build -mod=readonly -trimpath -buildvcs=false \
    -ldflags=-buildid= -o "$output/bin/nhop-node" ./cmd/nhop-node
  shasum -a 256 --check "$output/source.sha256"
) > "$output/build.log" 2>&1
go version -m "$output/bin/nhop-node" > "$output/program-build.txt"
expected="$(awk -F '\t' -v target="$arch" '$1 == target { print $2 }' "$root/ops/testing/acceptance/installer/programs.tsv")"
[[ "$expected" =~ ^[0-9a-f]{64}$ && "$(shasum -a 256 "$output/bin/nhop-node" | awk '{print $1}')" == "$expected" ]] || exit 2
printf 'project_commit=%s\narchive_sha256=%s\ntarget=linux/%s\n' "$previous_commit" "$previous_sha" "$arch" > "$output/provenance.txt"
shasum -a 256 "$output/bin/nhop-node" > "$output/program.sha256"
printf 'built\n' > "$output/status"
