#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
fleet_lab_environment
fleet_lab_owned
mode="${1:-canary}"
export NHOP_FLEET_RECOVERY_CYCLES="${2:-12}"
case "$mode" in
  canary) pattern='^TestFleetCanary$' ;;
  tuic-recovery) pattern='^TestFleetTUICRecovery$' ;;
  *) printf 'Unknown Fleet lab scenario.\n' >&2; exit 2 ;;
esac
[[ "$NHOP_FLEET_RECOVERY_CYCLES" =~ ^[0-9]+$ && "$NHOP_FLEET_RECOVERY_CYCLES" -ge 1 && "$NHOP_FLEET_RECOVERY_CYCLES" -le 100 ]] || exit 2
for node in "${fleet_nodes[@]}"; do
  [[ "$(fleet_lab_shell "$node" hostname)" == "lima-$node" ]] || exit 2
done
run="$(mktemp -d "$fleet_data/runs/run-XXXXXXXX")"
export NHOP_FLEET_LAB_ROOT="$run" NHOP_FLEET_LAB_REPOSITORY="$lab_root"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags=-buildid= \
  -o "$run/nhop-fleet" ./cmd/nhop-fleet
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false -ldflags=-buildid= \
  -o "$run/canary.test" ./internal/control/fleet/canary
{
  go version
  git rev-parse HEAD
  git status --porcelain --untracked-files=normal
  "$lab_lima" --version
  "$lab_lima" network list --json
  "$lab_lima" list --json "${fleet_nodes[@]}"
  printf 'scenario=%s\nrecovery_cycles=%s\n' "$mode" "$NHOP_FLEET_RECOVERY_CYCLES"
} > "$run/provenance.txt"
find cmd internal ops/testing/linux -type f \( -name '*.go' -o -name '*.html' -o -name '*.css' -o -name '*.sh' -o -name '*.tsv' \) -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 > "$run/source.sha256"
shasum -a 256 go.mod go.sum "$run/nhop-fleet" "$run/canary.test" >> "$run/source.sha256"
set +e
CGO_ENABLED=1 go test -race -count=1 -v -timeout=30m -run "$pattern" ./internal/control/fleet/canary 2>&1 | tee "$run/host.log"
result="${PIPESTATUS[0]}"
set -e
(cd "$run"; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$run/evidence.sha256"
printf 'Three-VM Fleet evidence: %s\n' "$run"
exit "$result"
