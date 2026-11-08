#!/usr/bin/env bash
# Sourced by run.sh: package and execute each independent private network fixture.
client_scenario() {
  local kind="$1" policy="$2" mode result=0 file binary=network.test
  mode="$report/$policy"
  if [[ "$kind" == systemd ]]; then mode="$report/service/$policy"; fi
  if [[ "$kind" == protocols ]]; then mode="$report/protocols/$policy"; fi
  if [[ "$kind" == installed ]]; then mode="$report/installed/$policy"; fi
  if [[ "$kind" == boot ]]; then mode="$report/boot/$policy"; fi
  if [[ "$kind" == install || "$kind" == boot-order ]]; then
    mode="$report/registration/$kind"
    binary="$kind.test"
  fi
  mkdir -m 0700 "$mode" "$mode/bin"
  cp "$report/$binary" "$mode/network.test"
  cp ops/testing/client/kernel-guest.sh "$mode/guest.sh"
  cp ops/testing/client/resolved-guest.sh "$mode/resolved-guest.sh"
  if [[ "$kind" != boot-order ]]; then
    cp "build/runtime-candidates/client-xray-linux-$arch/bin/xray" "$mode/bin/"
    cp "build/runtime-candidates/client-sing-box-linux-$arch/bin/"* "$mode/bin/"
  fi
  if [[ "$kind" == systemd || "$kind" == installed ]]; then
    cp ops/testing/client/systemd-guest.sh "$mode/systemd-guest.sh"
  fi
  if [[ "$kind" == installed ]]; then
    cp ops/testing/client/install/managed-guest.sh "$mode/installed-guest.sh"
  fi
  cp "$report/nhop-client" "$mode/bin/nhop-client"
  (cd "$mode"; sha256sum network.test guest.sh *-guest.sh bin/*) > "$mode/inputs.sha256"
  client_host_baseline > "$mode/host-before.txt" || return 1
  (cd "$mode"; sudo -n env GITHUB_ACTIONS=true GITHUB_REPOSITORY="$GITHUB_REPOSITORY" NHOP_CLIENT_CI="linux/$arch" \
    GORACE=atexit_sleep_ms=0 unshare --mount --net bash guest.sh "$kind" "$policy") > "$mode/guest.log" 2>&1 || result=$?
  for file in bus.log resolved.log systemd-unit.txt systemd-unit.sha256 systemd-override.txt installed-snapshot.json \
    installed-profile.json installed-runtimes.json installed-release.json installed-owner.json; do
    if [[ -e "$mode/$file" ]]; then
      [[ -f "$mode/$file" && ! -L "$mode/$file" ]] || return 2
      sudo -n chown "$(id -u):$(id -g)" "$mode/$file"
    fi
  done
  client_host_baseline > "$mode/host-after.txt" || return 1
  cmp "$mode/host-before.txt" "$mode/host-after.txt" || return 1
  ((result == 0)) || { cat "$mode/guest.log"; return "$result"; }
  awk '/WARNING: DATA RACE/ { bad=1 } END { exit bad }' "$mode/guest.log" || return 1
  (cd "$mode"; sha256sum --check inputs.sha256) > "$mode/input-integrity.log" || return 1
  printf 'passed\n' > "$mode/status"
}

client_host_baseline() {
  sha256sum /etc/passwd /etc/group /etc/ssl/certs/ca-certificates.crt /etc/resolv.conf || return 1
  ip -4 -j rule || return 1
  ip -6 -j rule || return 1
  findmnt -n -o OPTIONS --target /run || return 1
}
