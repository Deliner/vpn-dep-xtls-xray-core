#!/usr/bin/env bash
set -euo pipefail
umask 077

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
recipe="$root/ops/runtime/cronet"
source "$recipe/prepare.sh"
source "$recipe/toolchain.sh"
source "$recipe/inspection.sh"
target="${1:-}"
output="${2:-}"
[[ "$(go env GOVERSION)" == go1.27.3 ]] || exit 2
case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) native_host=mac-arm64; native_ninja=ninja-mac ;;
  Linux/x86_64) native_host=linux-amd64; native_ninja=ninja-linux ;;
  *) printf 'Native build requires macOS ARM64 or Linux AMD64 host.\n' >&2; exit 2 ;;
esac
case "$target" in linux/arm64) native_os=linux; native_cpu=arm64 ;; linux/amd64) native_os=linux; native_cpu=x64 ;; darwin/arm64) native_os=mac; native_cpu=arm64 ;; *) exit 2 ;; esac
[[ "$native_os" != mac || "$native_host" == mac-arm64 ]] || { printf 'Darwin target requires macOS SDK.\n' >&2; exit 2; }
[[ "$output" =~ ^build/native-candidates/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || { printf 'Expected a new build/native-candidates/name\n' >&2; exit 2; }
for directory in "$root/build" "$root/build/native-candidates"; do
  [[ ! -L "$directory" ]] || exit 2
  mkdir -p "$directory"
done
work="$root/$output"
[[ ! -e "$work" && ! -L "$work" ]] || { printf 'Native output already exists\n' >&2; exit 2; }
mkdir "$work" "$work/inputs" "$work/bin"
trap 'printf "failed\n" > "$work/status"' ERR
printf 'preparing\n' > "$work/status"
native_archive_source cronet-go "$work/source"
native_archive_source naiveproxy "$work/source/naiveproxy"
src="$work/source/naiveproxy/src"
native_prepare_tools
native_prepare_target
export PATH="$work/tools:$PATH"
export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOPRIVATE= GONOPROXY= GONOSUMDB=
{
  printf 'target=%s\nninja_jobs=4\n' "$target"
  go version
  "$src/third_party/llvm-build/Release+Asserts/bin/clang" --version
  "$src/gn/out/gn" --version
  "$work/tools/ninja-real" --version
  python3 --version
  if [[ "$native_host" == mac-arm64 ]]; then xcrun --sdk macosx --show-sdk-version; else uname -a; fi
  (cd "$root" && shasum -a 256 ops/runtime/cronet/*)
  (cd "$work" && shasum -a 256 inputs/* source.tsv inputs.tsv tools/*)
} > "$work/provenance.txt"
printf 'building\n' > "$work/status"
go -C "$work/source" run ./cmd/build-naive build --target "$target" > "$work/build.log" 2>&1
native_output="$src/out/cronet-$native_os-$native_cpu"
cp "$native_output/obj/components/cronet/libcronet_static.a" "$work/bin/libcronet.a"
if [[ "$native_os" == linux ]]; then cp "$native_output/libcronet.so" "$work/bin/libcronet.so"; fi
native_inspect
(cd "$work" && shasum -a 256 bin/* provenance.txt build.log inspection/* > SHA256SUMS)
printf 'native-built\n' > "$work/status"
printf 'Native candidate built; ABI and payload acceptance still required: %s\n' "$work"
