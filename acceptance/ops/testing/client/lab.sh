#!/usr/bin/env bash
# Host-side packaging/transport; network mutation belongs only to the guest runner.
source "$(dirname "${BASH_SOURCE[0]}")/../linux/environment.sh"

client_lab_begin() {
  lab_environment
  lab_owned
  mkdir -p "$lab_data/client-kernel"
  run="$(mktemp -d "$lab_data/client-kernel/run-XXXXXXXX")"
  printf 'preparing\n' > "$run/status"
  trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
  GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false \
    -o "$run/network.test" ./internal/control/client/network
  cp "$lab_root/ops/testing/client/kernel-guest.sh" "$run/guest.sh"
  mkdir "$run/bin"
  {
    go version
    git -C "$lab_root" rev-parse HEAD
    git -C "$lab_root" status --porcelain
    (cd "$lab_root"; shasum -a 256 internal/control/client/network/*.go ops/testing/client/*.sh)
  } > "$run/provenance.txt"
}

client_lab_run() {
  local mode="$1" run_name digest
  (cd "$run"; shasum -a 256 network.test guest.sh provenance.txt; for binary in bin/*; do [[ -f "$binary" ]] || continue; shasum -a 256 "$binary"; done) > "$run/SHA256SUMS"
  COPYFILE_DISABLE=1 tar --no-xattrs -czf "$run/package.tar.gz" -C "$run" network.test guest.sh provenance.txt SHA256SUMS bin
  run_name="client-kernel-$(basename "$run")"
  lab_shell mkdir -p "runs/$run_name"
  "$lab_lima" copy "$run/package.tar.gz" "$lab_instance:/home/nhop/runs/$run_name/package.tar.gz"
  digest="$(shasum -a 256 "$run/package.tar.gz" | awk '{print $1}')"
  lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$run_name" "$digest" "$mode" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
cd "runs/$1"
printf '%s  package.tar.gz\n' "$2" | sha256sum --check
tar -xzf package.tar.gz
sha256sum --check SHA256SUMS
sudo -n unshare --mount --net bash guest.sh "$3"
GUEST
  printf 'passed\n' > "$run/status"
  cat "$run/guest.log"
  printf 'Isolated Linux client evidence: %s\n' "$run"
}
