#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)/$(uname -m)" == Linux/x86_64 ]] || exit 2
arch="$1"
case "$arch" in amd64) cpu=x64 ;; arm64) cpu=arm64 ;; *) exit 2 ;; esac
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
cd "$root"
report="$root/build/acceptance-results"
[[ ! -e "$report" && ! -L "$report" ]] || exit 2
mkdir -p "$report"
work="$root/build/native-candidates/cronet-linux-$arch"
preserve_cronet_result() {
  local result=$?
  if [[ -d "$work" ]]; then
    for name in build.log provenance.txt source.tsv inputs.tsv SHA256SUMS status inspection bin; do
      if [[ -e "$work/$name" ]]; then cp -R "$work/$name" "$report/"; fi
    done
  fi
  if ((result != 0)); then printf 'failed\n' > "$report/status"; fi
  (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"
  return "$result"
}
trap preserve_cronet_result EXIT
printf 'preparing\n' > "$report/status"
{
  date -u +%FT%TZ
  uname -a
  lscpu
  cat /etc/os-release source.txt
  printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\n' "$arch" "${GITHUB_SHA:-local}" "${GITHUB_RUN_ID:-local}"
} > "$report/host.txt"
while IFS=$'\t' read -r component url revision tree; do
  [[ -n "$component" && "$component" != \#* ]] || continue
  repository="${url#https://github.com/}"
  just dependency-checkout "${repository%.git}" "$revision" >> "$report/source-fetch.log" 2>&1
done < ops/runtime/cronet/source.tsv
for component in clang-linux-amd64 gn-linux-amd64 ninja-linux pgo-linux sysroot-linux-amd64; do
  just cronet-review-input "$component" >> "$report/input-review.log" 2>&1
done
if [[ "$arch" == arm64 ]]; then just cronet-review-input sysroot-linux-arm64 >> "$report/input-review.log" 2>&1; fi
just cronet-build "linux/$arch" "build/native-candidates/cronet-linux-$arch" > "$report/driver.log" 2>&1
readelf --file-header --dynamic --dyn-syms --wide "$work/bin/libcronet.so" > "$work/inspection/elf.txt"
GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOFLAGS=-mod=readonly \
  go -C "$work/source/test" test -c -tags with_purego -trimpath -buildvcs=false -o "$work/bin/range.test" .
(cd "$work"; shasum -a 256 bin/range.test) >> "$work/SHA256SUMS"
printf 'Native build only; ABI execution, CVE triage and payload acceptance remain separate.\n' > "$report/limitations.txt"
