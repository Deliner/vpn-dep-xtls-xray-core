#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$1" =~ ^build/reports/cronet-components/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
recipe="$root/ops/testing/cronet/components"
source "$root/ops/testing/cronet/source.sh"
cronet_verify_source
for directory in "$root/build" "$root/build/reports" "$root/build/reports/cronet-components"; do
  [[ ! -L "$directory" ]] || exit 2
  mkdir -p "$directory"
done
work="$root/$1"
mkdir "$work" "$work/inputs" "$work/source" "$work/bin" "$work/logs"
printf 'running\n' > "$work/status"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
cp "$recipe/"* "$work/inputs/"
cp "$root/ops/testing/cronet/source.sh" "$work/inputs/source.sh"
cp "$root/ops/runtime/cronet/source.tsv" "$work/source.tsv"
git -C "$checkout" archive "$revision" src/base/third_party/icu src/base/third_party/superfasthash > "$work/inputs/components.tar"
tar -xf "$work/inputs/components.tar" -C "$work/source"
[[ -z "$(find "$work/source" -type l -print)" ]] || exit 1
{
  printf 'revision=%s\ntree=%s\nsource_tests_only=true\n' "$revision" "$tree"
  uname -sm
  clang --version
  clang -dumpmachine
} > "$work/provenance.txt"
flags=(-O1 -g -fsanitize=address,undefined -fno-sanitize-recover=all -fno-omit-frame-pointer)
command=(clang "${flags[@]}" "$work/inputs/hash.c" "$work/source/src/base/third_party/superfasthash/superfasthash.c" -o "$work/bin/hash")
printf '%q ' "${command[@]}" > "$work/logs/hash.command"
"${command[@]}" > "$work/logs/hash.build" 2>&1
command=(clang++ -std=c++17 "${flags[@]}" "-I$work/source/src/base/third_party/icu" "$work/inputs/utf.cc" -o "$work/bin/utf")
printf '%q ' "${command[@]}" > "$work/logs/utf.command"
"${command[@]}" > "$work/logs/utf.build" 2>&1
export ASAN_OPTIONS=halt_on_error=1:abort_on_error=0:exitcode=86
export UBSAN_OPTIONS=halt_on_error=1:exitcode=86
printf 'ASAN_OPTIONS=%s\nUBSAN_OPTIONS=%s\n' "$ASAN_OPTIONS" "$UBSAN_OPTIONS" >> "$work/provenance.txt"
for component in hash utf; do
  "$work/bin/$component" > "$work/logs/$component.stdout" 2> "$work/logs/$component.stderr" || exit 1
  [[ ! -s "$work/logs/$component.stderr" ]] || exit 1
  code=0
  "$work/bin/$component" --bounds-control > "$work/logs/$component-control.stdout" 2> "$work/logs/$component-control.stderr" || code=$?
  printf '%s\n' "$code" > "$work/logs/$component-control.exit"
  [[ "$code" == 86 ]] || exit 1
  # Clang can diagnose the inline UTF load with UBSan before ASan sees it.
  if ! grep -q 'ERROR: AddressSanitizer: heap-buffer-overflow' "$work/logs/$component-control.stderr"; then
    [[ "$component" == utf ]] || exit 1
    grep -Fq "$work/inputs/utf.cc:" "$work/logs/$component-control.stderr" || exit 1
    grep -q 'runtime error: load of address .* with insufficient space for an object' "$work/logs/$component-control.stderr" || exit 1
  fi
done
[[ "$(cat "$work/logs/hash.stdout")" == hash_bounds_cases=32770 ]] || exit 1
[[ "$(cat "$work/logs/utf.stdout")" == 'utf8_buffers=196864 utf16_buffers=131072 scalar_roundtrips=1112064' ]] || exit 1
cronet_verify_source
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$work/evidence.sha256"
cat "$work/logs/"{hash,utf}.stdout
printf 'Source-only component bounds tests: %s\n' "$work"
