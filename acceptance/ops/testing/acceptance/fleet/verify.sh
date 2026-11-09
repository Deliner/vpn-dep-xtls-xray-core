#!/usr/bin/env bash
# Strict lifecycle/traffic contract; callers also verify every retained executable.
fleet_verify_report() {
  jq -e --arg platform "linux/$2" '
    .version == 1 and .status == "passed" and .platform == $platform and
    .rollback == true and .agent_stop_retains_vpn == true and
    .controller_recovery == true and .agent_crash_recovery == true and
    .credentials_unchanged == true and .revoke_exit_status == "78" and
    .revoke_no_restart == true and .removed_state_retained == true and
    (.generations | length == 3 and (unique | length) == 3 and all(.[]; type == "string" and test("^[0-9a-f]{64}$"))) and
    .installer_interruption.recovered == true and
    .controller_crash.recovered == true and
    (.traffic | keys == ["agent-restarted","controller-crashed","controller-offline","controller-restarted","initial","interrupted-install-recovered","revoked-agent-vpn-retained","rolled-back-agent-stopped","updated"]) and
    all(.traffic | to_entries[]; .key == .value.phase and
      .value.profiles == 2 and .value.required_rounds == 3 and .value.barrier_rounds == 3 and
      .value.concurrent_tcp_udp_streams == 4 and
      ((.value.missing_arrivals // []) | length == 0) and
      (.value.routes | keys == ["shadowsocks","tuic"]) and
      all(.value.routes[]; .tcp_rounds == 3 and .udp_rounds == 3 and (.error // "") == ""))
  ' "$1" > /dev/null
}

fleet_verify_build() {
  local binary="$1" arch="$2" observed="$3" machine
  go version -m "$binary" > "$observed" || return 1
  for machine in '-race=true' 'CGO_ENABLED=1' 'GOOS=linux' "GOARCH=$arch"; do
    grep -Fq -- "$machine" "$observed" || return 1
  done
  case "$arch" in amd64) machine=x86-64 ;; arm64) machine=aarch64 ;; *) return 2 ;; esac
  file -b "$binary" | grep -q "ELF 64-bit LSB.*$machine" || return 1
}
