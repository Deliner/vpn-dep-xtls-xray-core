#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 2 && "$(uname -s)" == Linux ]] || exit 2
case "$1/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) printf 'Native test CPU differs from target\n' >&2; exit 2 ;; esac
candidate="$(cd "$2" && pwd -P)"
[[ "$(cat "$candidate/status")" == native-built ]] || exit 2
(cd "$candidate"; sha256sum --check evidence.sha256; sha256sum --check SHA256SUMS)
# GitHub's artifact ZIP does not retain executable mode; restore only this verified binary.
chmod u+x "$candidate/bin/range.test"
report="$PWD/build/cronet-native-verification"
mkdir "$report"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
{
  uname -a
  lscpu
  printf 'workflow_sha=%s\nrun_id=%s\n' "${GITHUB_SHA:-local}" "${GITHUB_RUN_ID:-local}"
  sha256sum "$candidate/bin/libcronet.so" "$candidate/bin/range.test" "$candidate/source.tsv"
} > "$report/identity.txt"
NHOP_CRONET_LIBRARY="$candidate/bin/libcronet.so" "$candidate/bin/range.test" \
  -test.run '^TestRangeResponseBoundaries$' -test.v -test.timeout 60s > "$report/range.log" 2>&1
printf 'passed\n' > "$report/status"
cat "$report/range.log"
