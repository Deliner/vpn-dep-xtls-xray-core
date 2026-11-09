#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
case "$3" in build/runtime-releases/*) ;; *) exit 2 ;; esac
name="${3#build/runtime-releases/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/runtime-releases" ]] || exit 2
mkdir -p "$root/build/runtime-releases"
cd "$root"
go run ./cmd/runtimepack stage ops/release/runtime/xray.json "$1" "$2" "$3"
