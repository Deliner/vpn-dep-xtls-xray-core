#!/usr/bin/env bash
# Bind a real killed controller to persistent authority, signed bytes and revoked access.
fleet_verify_controller() (
  set -euo pipefail
  local directory="$1" name digest
  for name in controller-before.json controller-after.json controller-crash-exit.json \
    controller-before-assignment.json controller-after-assignment.json controller-1.log controller-2.log controller-3.log; do
    [[ -f "$directory/$name" && ! -L "$directory/$name" ]] || exit 1
  done
  for name in before after; do
    digest="$(shasum -a 256 "$directory/controller-$name.json" | awk '{print $1}')"
    jq -e --arg field "${name}_sha256" --arg digest "$digest" '.controller_crash[$field] == $digest' "$directory/fleet.json" > /dev/null || exit 1
    digest="$(shasum -a 256 "$directory/controller-$name-assignment.json" | awk '{print $1}')"
    jq -e --arg digest "$digest" '.assignment_sha256 == $digest and .active.desired.envelope_sha256 == $digest' "$directory/controller-$name.json" > /dev/null || exit 1
  done
  cmp "$directory/controller-before-assignment.json" "$directory/controller-after-assignment.json" || exit 1
  jq -e -s '
    .[0] as $report | $report.controller_crash as $p | .[1] as $b | .[2] as $a | .[3] as $exit |
    $p.recovered == true and $p.offline_verified == true and $p.signaled == true and
    $p.pid == $b.pid and $exit.pid == $b.pid and $p.signal == "SIGKILL" and $exit.signal == "SIGKILL" and
    $p.signal_number == 9 and $exit.signal_number == 9 and $exit.signaled == true and
    $b.pid > 1 and $a.pid > 1 and $b.pid != $a.pid and
    ($b.agent_pid | test("^[1-9][0-9]*$")) and $b.agent_pid == $a.agent_pid and
    ($b.vpn_pid | test("^[1-9][0-9]*$")) and $b.vpn_pid == $a.vpn_pid and
    $b.hashes == $a.hashes and
    ($b.hashes | keys == ["admin","agent_credentials","authority","executable","tls_certificate","tls_key"]) and
    all($b.hashes[]; type == "string" and test("^[0-9a-f]{64}$")) and
    $b.hashes.executable == $report.agent_sha256 and
    $b.active.desired == $a.active.desired and $b.assignment_sha256 == $a.assignment_sha256 and
    $b.revoked == $a.revoked and $b.revoked.enrolled == true and $b.revoked.revoked == true and
    $b.revoked.node_id == "revoked-control" and $b.revoked.fleet_id == "acceptance" and
    all([$b,$a][]; .active.enrolled == true and .active.revoked == false and
      .active.fleet_id == "acceptance" and .active.node_id == "fleet-node" and
      .active.status.phase == "ready" and .active.status.accepted_revision == .active.desired.revision and
      .active.status.applied_revision == .active.desired.revision and
      .active.status.generation == .active.desired.generation and
      .active.desired.generation == $report.generations[-1] and
      .revoked_assignment_http == 403 and .revoked_report_http == 403 and .revoked_enroll_http == 403) and
    ($a.active.status.attempt | tonumber) > ($b.active.status.attempt | tonumber)
  ' "$directory/fleet.json" "$directory/controller-before.json" "$directory/controller-after.json" \
    "$directory/controller-crash-exit.json" > /dev/null || exit 1
)
