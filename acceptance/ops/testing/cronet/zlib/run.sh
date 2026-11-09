#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd -P)"
recipe="$root/ops/testing/cronet/zlib"
source "$recipe/source.sh"

window_build() {
  local variant="$1" implementation="$2" src="$work/$1" output="$work/bin/$1-$2"
  local decoder=("$src/inflate.c" "$src/inffast.c") flags=(-DZLIB_IMPLEMENTATION)
  if [[ "$implementation" == optimized ]]; then
    decoder=("$src/contrib/optimizations/inflate.c" "$src/contrib/optimizations/inffast_chunk.c")
    flags+=("${simd[@]}" -DINFLATE_CHUNK_READ_64LE -DDEFLATE_CHUNK_WRITE_64LE)
  fi
  local command=(clang -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer
    -fno-sanitize-recover=all "${flags[@]}" "-I$src" "$work/inputs/window.c"
    "${decoder[@]}" "$src/inftrees.c" "$src/zutil.c" "$src/adler32.c" "$src/crc32.c" -o "$output")
  printf '%q ' "${command[@]}" > "$work/logs/$variant-$implementation.command"
  printf '\n' >> "$work/logs/$variant-$implementation.command"
  "${command[@]}" > "$work/logs/$variant-$implementation.build" 2>&1
}

window_case() {
  local binary="$1" bits="$2" expected="$3" retained="$4" length="$5" code=0
  local name="$1-$2" observation
  ASAN_OPTIONS=halt_on_error=1 UBSAN_OPTIONS=halt_on_error=1 \
    "$work/bin/$binary" "$bits" > "$work/logs/$name.stdout" 2> "$work/logs/$name.stderr" || code=$?
  observation="requested_bits=$bits retained_bits=$retained output=40960 dictionary=$length"
  [[ "$code" == "$expected" && ! -s "$work/logs/$name.stderr" ]] || return 1
  [[ "$(cat "$work/logs/$name.stdout")" == "$observation" ]] || return 1
  printf '%s\t%s\t%s\t%s\n' "$binary" "$bits" "$code" "$observation" >> "$work/results.tsv"
}

relative="$1"
[[ "$relative" =~ ^build/reports/cronet-zlib/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || exit 2
for directory in "$root/build" "$root/build/reports" "$root/build/reports/cronet-zlib"; do
  [[ ! -L "$directory" ]] || exit 2
  mkdir -p "$directory"
done
work="$root/$relative"
[[ ! -e "$work" && ! -L "$work" ]] || exit 2
case "$(uname -s)/$(uname -m)" in
  Darwin/arm64|Linux/aarch64) simd=(-DINFLATE_CHUNK_SIMD_NEON) ;;
  Linux/x86_64) simd=(-DINFLATE_CHUNK_SIMD_SSE2 -msse3) ;;
  *) printf 'Supported source-test hosts: Darwin ARM64, Linux ARM64/AMD64.\n' >&2; exit 2 ;;
esac
mkdir "$work" "$work/inputs" "$work/baseline" "$work/patched" "$work/bin" "$work/logs"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi; exit "$result"' EXIT
printf 'running\n' > "$work/status"
window_prepare_source
printf 'binary\trequested_bits\texit_code\tobservation\n' > "$work/results.tsv"
for implementation in generic optimized; do
  window_build baseline "$implementation"
  window_build patched "$implementation"
  window_case "baseline-$implementation" 0 1 23 40960
  window_case "baseline-$implementation" 15 0 15 32768
  window_case "patched-$implementation" 0 0 0 32768
  window_case "patched-$implementation" 15 0 15 32768
done
cronet_verify_source
printf 'passed\n' > "$work/status"
(cd "$work"; find . -type f ! -name evidence.sha256 -exec shasum -a 256 '{}' \; | LC_ALL=C sort) > "$work/evidence.sha256"
cat "$work/results.tsv"
printf 'Source regression passed (not a Cronet ABI/network test): %s\n' "$work"
