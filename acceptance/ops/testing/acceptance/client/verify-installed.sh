#!/usr/bin/env bash
# Independently bind actual installed inputs to the tested CLI/runtime archives.
client_verify_installed() {
  local mode="$1" policy="$2" line
  client_verify_recovery "$mode" "$policy" TestKernelInstalledServiceManagedRecovery || return 1
  client_verify_service_unit "$mode" "$policy" '^/etc/systemd/system/nhop-client\.service$' || return 1
  go run ./cmd/clientcheck "$mode" "$policy" || return 1
  client_verify_installed_context "$mode" || return 1
  for line in 'original installation profile/inventory removed before service start' \
    'installed service: actual CLI, immutable runtime/profile snapshot, TCP/UDP/IPv6/DNS and retained guard recovery verified' \
    'installed fixture removed: unit, generation and isolation drop-in absent'; do
    awk -v expected="$line" 'index($0,expected) { count++ } END { exit count != 1 }' "$mode/guest.log" || return 1
  done
}

client_verify_installed_context() {
  local mode="$1" pid root
  pid="$(sed -nE 's@^NetworkNamespacePath=/proc/([0-9]+)/ns/net$@\1@p' "$mode/systemd-override.txt")"
  root="$(sed -nE 's@^BindPaths=(/run/nhop-service-test-[0-9]+)/session/etc:/etc$@\1@p' "$mode/systemd-override.txt")"
  [[ "$pid" =~ ^[1-9][0-9]+$ && "$root" =~ ^/run/nhop-service-test-[0-9]+$ ]] || return 1
  {
    printf '[Service]\n\n# Test-only private namespace context; production settings above are unchanged.\nNetworkNamespacePath=/proc/%s/ns/net\n' "$pid"
    printf 'BindPaths=%s/session/etc:/etc\nBindPaths=%s/control:/run/nhop-client\nBindPaths=%s/session:/run/nhop-session-test\n' "$root" "$root" "$root"
    printf 'BindPaths=%s/resolve:/run/systemd/resolve\nBindPaths=%s/netif:/run/systemd/netif\n' "$root" "$root"
    printf 'Environment=DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/nhop-session-test/bus GORACE=atexit_sleep_ms=0\nRuntimeMaxSec=90s\n'
  } | cmp - "$mode/systemd-override.txt" || return 1
}
