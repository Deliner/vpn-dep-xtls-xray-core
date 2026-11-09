#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 4 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
case "$1" in amd64|arm64) ;; *) exit 2 ;; esac
case "$4" in build/runtime-releases/*) ;; *) printf 'Output must be under build/runtime-releases/\n' >&2; exit 2 ;; esac
name="${4#build/runtime-releases/}"
case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/runtime-releases" ]] || exit 2
mkdir -p "$root/build/runtime-releases"
cd "$root"
go run ./cmd/runtimepack package ops/release/runtime/xray.json "$1" "$2" "$3" "$4"
