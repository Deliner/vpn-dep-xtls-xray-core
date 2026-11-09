#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "${GITHUB_ACTIONS:-}" == true ]] || exit 2
[[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core ]] || exit 2
arch="$1"
case "$arch/$(uname -m)" in amd64/x86_64|arm64/aarch64) ;; *) exit 2 ;; esac
root="$PWD"
source "$root/ops/runtime/integrity.sh"
source "$root/ops/testing/acceptance/client/scenarios.sh"
source "$root/ops/testing/acceptance/client/release.sh"
report="$root/build/client-acceptance"
mkdir -m 0700 "$report"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; (cd "$report"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"' EXIT
printf 'running\n' > "$report/status"
{
  printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\n' "$arch" "${GITHUB_SHA:?}" "${GITHUB_RUN_ID:?}"
  uname -a
  lscpu
  go version
  cc --version
  dpkg-query --show nftables iproute2 dbus systemd-resolved ca-certificates jq
  systemctl --version
  [[ "$(cat /proc/1/comm)" == systemd ]]
} > "$report/identity.txt"
for engine in xray sing-box; do
  output="build/runtime-candidates/client-$engine-linux-$arch"
  just candidate-build "$engine" linux "$arch" "$output" > "$report/$engine-build.log" 2>&1
  candidate_complete "$output"
  candidate_inventory "$output" > "$report/$engine-observed.sha256"
  cmp "$output/SHA256SUMS" "$report/$engine-observed.sha256"
  cp "$output/provenance.txt" "$report/$engine-provenance.txt"
  cp "$output/SHA256SUMS" "$report/$engine-SHA256SUMS"
done
mkdir "$report/release-policy"
cp ops/release/runtime/{xray,sing-box}.json "$report/release-policy/"
client_verify_release_runtime "$report/release-policy" "$arch" \
  "build/runtime-candidates/client-xray-linux-$arch/bin/xray" \
  "build/runtime-candidates/client-sing-box-linux-$arch/bin/sing-box" \
  "build/runtime-candidates/client-sing-box-linux-$arch/bin/libcronet.so" > "$report/release-runtime-verification.log"
CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go test -race -c -trimpath -buildvcs=false \
  -o "$report/network.test" ./internal/control/client/acceptance
go version -m "$report/network.test" > "$report/test-build.txt"
CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -race -trimpath -buildvcs=false \
  -o "$report/nhop-client" ./cmd/nhop-client
go version -m "$report/nhop-client" > "$report/client-build.txt"
for kind in install boot-order; do
  package=./internal/control/client/systemd
  if [[ "$kind" == install ]]; then package+=/install; fi
  CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go test -race -c -trimpath -buildvcs=false \
    -o "$report/$kind.test" "$package"
  go version -m "$report/$kind.test" > "$report/$kind-build.txt"
done
mkdir -m 0700 "$report/service" "$report/protocols" "$report/installed" "$report/boot" "$report/registration"
client_scenario install tunnel
client_scenario boot-order tunnel
for policy in tunnel block; do
  client_scenario boot "$policy"
  client_scenario public "$policy"
  client_scenario systemd "$policy"
  client_scenario installed "$policy"
  client_scenario protocols "$policy"
done
printf 'passed\n' > "$report/status"
