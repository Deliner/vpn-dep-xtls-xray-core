#!/usr/bin/env bash
# Match raw pre/post-crash evidence to the exact accepted assignment and installer transaction.
fleet_verify_interruption() (
  set -euo pipefail
  local directory="$1" name field digest
  for name in interrupted-transaction.json interrupted-agent-before.json interrupted-agent-killed.json \
    interrupted-agent-after.json interrupted-desired.json interrupted-final-state.json interrupted-systemd-killed.txt; do
    [[ -f "$directory/$name" && ! -L "$directory/$name" ]] || exit 1
  done
  for name in interrupted-transaction interrupted-agent-before interrupted-agent-after; do
    case "$name" in interrupted-transaction) field=journal_sha256 ;; interrupted-agent-before) field=intent_before_sha256 ;; *) field=intent_after_sha256 ;; esac
    digest="$(shasum -a 256 "$directory/$name.json" | awk '{print $1}')"
    jq -e --arg field "$field" --arg digest "$digest" '.installer_interruption[$field] == $digest' "$directory/fleet.json" > /dev/null || exit 1
  done
  cmp "$directory/interrupted-agent-before.json" "$directory/interrupted-agent-killed.json" || exit 1
  for field in MainPID=0 ExecMainStatus=9 Result=signal; do
    grep -Fxq "$field" "$directory/interrupted-systemd-killed.txt" || exit 1
  done
  jq -e -s '
    .[0].installer_interruption as $p | .[1] as $b | .[2] as $a | .[3] as $d | .[4] as $j | .[5] as $state |
    $p.recovered == true and $p.signal == "SIGKILL" and $p.exit_status == "9" and $p.unit_result == "signal" and
    ($p.agent_pid_before | test("^[1-9][0-9]*$")) and ($p.agent_pid_after | test("^[1-9][0-9]*$")) and
    $p.agent_pid_before != $p.agent_pid_after and $p.systemd_control_pid > 1 and
    ($p.credentials_before_sha256 | test("^[0-9a-f]{64}$")) and $p.credentials_before_sha256 == $p.credentials_after_sha256 and
    $b.phase == "applying" and $a.phase == "ready" and
    $b.accepted == $a.accepted and $a.applied == $a.accepted and
    $b.accepted.revision == $p.revision and $p.revision == $d.revision and
    $b.accepted.payload_sha256 == $p.payload_sha256 and $p.payload_sha256 == $d.payload_sha256 and
    ($b.applied.revision | tonumber) > 0 and ($b.applied.revision | tonumber) < ($p.revision | tonumber) and
    $b.attempt == $p.attempt_before and $a.attempt == $p.attempt_after and
    ($a.attempt | tonumber) > ($b.attempt | tonumber) and
    $j.phase == "pending" and $j.old.release == ("/opt/vpn-nhop/releases/" + $p.previous_release) and
    $j.next.release == ("/opt/vpn-nhop/releases/" + $p.next_release) and $p.previous_release != $p.next_release and
    ($j.next.state | @base64d | fromjson) == $state and $state.release_id == $p.next_release and
    $state.generation == $d.generation
  ' "$directory/fleet.json" "$directory/interrupted-agent-before.json" "$directory/interrupted-agent-after.json" \
    "$directory/interrupted-desired.json" "$directory/interrupted-transaction.json" "$directory/interrupted-final-state.json" > /dev/null || exit 1
)
