#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 5 && ! -e "$5" && ! -L "$5" ]] || exit 2
root="$PWD"
policy="$1"; arch="$2"
candidate="$(cd "$3" && pwd -P)"
native="$(cd "$4" && pwd -P)"
go run ./cmd/runtimepack singbox-metadata-verify "$policy" "$arch" "$candidate" "$native"
[[ -z "$(find "$candidate" ! -type f ! -type d -print -quit)" ]] || exit 2
source_sha="$(sed -n 's/^source_archive_sha256=//p' "$candidate/provenance.txt")"
[[ "$source_sha" =~ ^[0-9a-f]{64}$ ]] || exit 2
printf '%s  %s\n' "$source_sha" "$candidate/source.tar.gz" | shasum -a 256 --check
mkdir "$5"
output="$(cd "$5" && pwd -P)"
work="$(mktemp -d "$root/build/singbox-recovery-XXXXXXXX")"
trap 'result=$?; rm -rf "$work"; if (( result != 0 )); then rm -rf "$output"; fi' EXIT
cp -R "$candidate/." "$output/"
# The full archive is the immutable, reviewed owned source input. Extract only
# into a new private directory; never overwrite the retained candidate files.
tar -xzf "$candidate/source.tar.gz" --strip-components=1 -C "$work"
while read -r digest name; do
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || exit 2
  [[ "$name" != /* && "$name" != *\\* && "$name" != *'/../'* && "$name" != *'/./'* ]] || exit 2
  if [[ -e "$output/$name" ]]; then continue; fi
  # This recovery addresses upload-artifact's hidden-file omission only.
  [[ "$name" == source/.* && "$name" != */.. && "$name" != */. ]] || exit 2
  original="$work/${name#source/}"
  [[ -f "$original" && ! -L "$original" ]] || exit 2
  printf '%s  %s\n' "$digest" "$original" | shasum -a 256 --check
  mkdir -p "$(dirname "$output/$name")"
  cp "$original" "$output/$name"
  printf '%s\t%s\n' "$digest" "$name" >> "$output/recovered-sources.tsv"
done < "$output/SHA256SUMS"
go run ./cmd/runtimepack singbox-verify "$policy" "$arch" "$output" "$native"
printf 'Restored hidden source entries against the native inventory: %s\n' "$output"
