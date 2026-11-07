#!/usr/bin/env bash
# Host-side packaging/transport; network mutation belongs only to the guest runner.
source "$(dirname "${BASH_SOURCE[0]}")/../linux/environment.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../../runtime/integrity.sh"

client_lab_begin() {
  lab_environment
  lab_owned
  mkdir -p "$lab_data/client-kernel"
  run="$(mktemp -d "$lab_data/client-kernel/run-XXXXXXXX")"
  printf 'preparing\n' > "$run/status"
  trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$run/status"; fi' EXIT
  GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false \
    -o "$run/network.test" "${1:-./internal/control/client/network}"
  cp "$lab_root/ops/testing/client/kernel-guest.sh" "$run/guest.sh"
  cp "$lab_root/ops/testing/client/resolved-guest.sh" "$run/resolved-guest.sh"
  mkdir "$run/bin"
  {
    go version
    git -C "$lab_root" rev-parse HEAD
    git -C "$lab_root" status --porcelain
    (cd "$lab_root"; find internal -name '*.go' -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256; shasum -a 256 go.mod go.sum cmd/nhop-client/main.go ops/testing/client/*.sh)
  } > "$run/provenance.txt"
}

client_lab_run() {
  local mode="$1" ipv6="${2:-tunnel}" run_name digest result
  (cd "$run"; shasum -a 256 network.test guest.sh resolved-guest.sh provenance.txt; for binary in bin/*; do [[ -f "$binary" ]] || continue; shasum -a 256 "$binary"; done) > "$run/SHA256SUMS"
  COPYFILE_DISABLE=1 tar --no-xattrs -czf "$run/package.tar.gz" -C "$run" network.test guest.sh resolved-guest.sh provenance.txt SHA256SUMS bin
  run_name="client-kernel-$(basename "$run")"
  lab_shell mkdir -p "runs/$run_name"
  "$lab_lima" copy "$run/package.tar.gz" "$lab_instance:/home/nhop/runs/$run_name/package.tar.gz"
  digest="$(shasum -a 256 "$run/package.tar.gz" | awk '{print $1}')"
  if lab_shell flock --nonblock /home/nhop/.payload.lock bash -s -- "$run_name" "$digest" "$mode" "$ipv6" > "$run/guest.log" 2>&1 <<'GUEST'
set -euo pipefail
cd "runs/$1"
printf '%s  package.tar.gz\n' "$2" | sha256sum --check
tar -xzf package.tar.gz
sha256sum --check SHA256SUMS
before_host_files="$(sha256sum /etc/passwd /etc/group /etc/ssl/certs/ca-certificates.crt /etc/resolv.conf)"
if sudo -n unshare --mount --net bash guest.sh "$3" "$4"; then result=0; else result=$?; fi
[[ "$(sha256sum /etc/passwd /etc/group /etc/ssl/certs/ca-certificates.crt /etc/resolv.conf)" == "$before_host_files" ]]
exit "$result"
GUEST
  then result=0; else result=$?; fi
  # Print only after the command's redirection has been unwound, never in EXIT.
  cat "$run/guest.log"
  if ((result != 0)); then return "$result"; fi
  printf 'passed\n' > "$run/status"
  printf 'Isolated Linux client evidence: %s\n' "$run"
}

client_lab_candidates() {
  local directory
  for directory in "$1" "$2"; do
    candidate_complete "$directory"
    [[ "$(sed -n 's/^target=//p' "$directory/provenance.txt")" == "linux/$lab_arch" ]] || return 2
    candidate_inventory "$directory" > "$run/observed.sha256"
    cmp "$run/observed.sha256" "$directory/SHA256SUMS"
  done
  rm "$run/observed.sha256"
  cp "$1/bin/xray" "$2/bin/sing-box" "$2/bin/libcronet.so" "$run/bin/"
}
