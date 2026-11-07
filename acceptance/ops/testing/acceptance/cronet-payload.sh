#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 3 && "$(uname -s)" == Linux ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) exit 2 ;; esac
candidate="$(cd "$2" && pwd -P)"
[[ "$(cat "$candidate/status")" == native-built ]] || exit 2
(cd "$candidate"; sha256sum --check evidence.sha256; sha256sum --check SHA256SUMS)
mkdir -m 0700 "$3"
report="$(cd "$3" && pwd -P)"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
mkdir "$report/bin"
for engine in xray sing-box; do
  output="build/cronet-payload-candidates/$engine-linux-$arch"
  just candidate-build "$engine" linux "$arch" "$output" > "$report/$engine-build.log" 2>&1
  cp "$output/provenance.txt" "$report/$engine-provenance.txt"
  cp "$output/SHA256SUMS" "$report/$engine-SHA256SUMS"
  cp "$output/bin/$engine" "$report/bin/$engine"
done
tags="$(sed -n 's/^build_tags=//p' "$report/sing-box-provenance.txt")"
case ",$tags," in *,with_purego,*) ;; *) printf 'Naive acceptance requires the dynamic Cronet loader.\n' >&2; exit 2 ;; esac
# Keep original candidates intact. This execution snapshot binds their Go
# engines to the newly built native library, with its own complete identity.
cp "$candidate/bin/libcronet.so" "$report/bin/"
cp "$candidate/source.tsv" "$report/cronet-source.tsv"
(cd "$report"; sha256sum bin/*) > "$report/runtime.sha256"
just payload "$report/bin/xray" "$report/bin/sing-box" \
  '^TestProtocolMatrixPayload$/(single|mixed-three-hop)/(naive-h2|naive-h3)$' "$report/payload" \
  > "$report/payload.log" 2>&1
for topology in single mixed-three-hop; do
  for protocol in naive-h2 naive-h3; do
    expected="--- PASS: TestProtocolMatrixPayload/$topology/$protocol "
    # A changed test selector must never turn zero executed cases into acceptance.
    if ! awk -v expected="$expected" 'index($0, expected) { found=1 } END { exit !found }' "$report/payload.log"; then
      printf 'Missing actual Naive payload case: %s/%s\n' "$topology" "$protocol" >&2
      exit 1
    fi
  done
done
(cd "$report"; sha256sum --check runtime.sha256)
printf 'passed\n' > "$report/status"
