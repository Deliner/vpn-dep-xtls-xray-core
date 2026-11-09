#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 && "$(uname -s)" == Linux && "$(id -u)" != 0 ]] || exit 2
arch="$1" root="$PWD"
export NHOP_NODE_CI="linux/$arch"
source "$root/ops/testing/diagnostics/netem/context.sh"
netem_native_identity
[[ "${GITHUB_WORKSPACE:-}" == /* && "$root" == "$GITHUB_WORKSPACE/acceptance" ]] || exit 2
source "$root/ops/runtime/integrity.sh"
source "$root/ops/testing/diagnostics/netem/runtime.sh"
source "$root/ops/testing/acceptance/masque/experiments.sh"
report="$root/build/masque-acceptance"
mkdir -m 0700 "$report"
trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$report/status"; fi; masque_evidence_inventory' EXIT
printf 'running\n' > "$report/status"
cp source.txt "$report/source.txt"
printf 'target=linux/%s\nworkflow_sha=%s\nrun_id=%s\nmachine=%s\n' \
  "$arch" "$GITHUB_SHA" "$GITHUB_RUN_ID" "$(uname -m)" > "$report/identity.txt"
{ uname -a; lscpu; go version; dpkg-query --show nftables iproute2 tcpdump; } > "$report/environment.txt"
candidate="$root/build/runtime-candidates/masque-xray-linux-$arch"
just candidate-build xray linux "$arch" "${candidate#"$root/"}" > "$report/build.log" 2>&1
just candidate-audit "${candidate#"$root/"}" > "$report/audit.log" 2>&1
candidate_complete "$candidate"
candidate_inventory "$candidate" > "$report/runtime-observed.sha256"
cmp "$candidate/SHA256SUMS" "$report/runtime-observed.sha256"
cp -R "$candidate" "$report/runtime"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go test -c -trimpath -buildvcs=false \
  -o "$report/network.test" ./internal/control/diagnostics/masque
go version -m "$report/network.test" > "$report/network-build.txt"
mkdir -m 0700 "$report/runs"
printf '0\tdefault\n0\treno\n1\treno\n1\tdefault\n' > "$report/plan.tsv"
before="$(netem_host_network)"
printf '%s\n' "$before" > "$report/host-before.txt"
while IFS=$'\t' read -r round policy; do masque_experiment "$round" "$policy"; done < "$report/plan.tsv"
netem_host_network > "$report/host-after.txt"
cmp "$report/host-before.txt" "$report/host-after.txt"
candidate_inventory "$candidate" > "$report/runtime-after.sha256"
cmp "$report/runtime-observed.sha256" "$report/runtime-after.sha256"
printf 'collected\n' > "$report/status"
masque_evidence_inventory
just acceptance-masque-verify "$report" "$arch" "$GITHUB_SHA" "$root" > "$root/build/masque-verification.log" 2>&1
cp "$root/build/masque-verification.log" "$report/verification.log"
printf 'passed\n' > "$report/status"
