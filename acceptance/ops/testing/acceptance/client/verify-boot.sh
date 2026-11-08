#!/usr/bin/env bash
# Early protection requires all transitions and independently checked leak/DNS observations.
client_verify_boot() {
  local mode="$1" policy="$2" scenario expected
  grep -q "bootstrap policy: IPv6=$policy$" "$mode/guest.log" || return 1
  for scenario in stop handoff before after; do
    expected="--- PASS: TestKernelBootProtectionHandoff/$scenario "
    awk -v expected="$expected" 'index($0, expected) { count++ } END { exit count != 1 }' "$mode/guest.log" || return 1
  done
  awk '/bootstrap mode=/ {
    value=$0; sub(/^.*bootstrap /,"",value); if (split(value,f,/ +/)!=5) bad=1
    mode=f[1]; sub(/^mode=/,"",mode)
    attempts=f[4]; sub(/^attempts=/,"",attempts)
    if (f[1] !~ /^mode=(stop|handoff|before|after)$/ || f[2]!="direct" || f[3]!="UDP:" || f[4] !~ /^attempts=[0-9]+$/ || attempts+0<20 || f[5]!="delivered=0") bad=1
    seen[mode]++
  }
  /^--- PASS: TestKernelBootProtectionHandoff \(/ { root++ }
  /^PASS$/ { complete++ }
  /--- (FAIL|SKIP):/ || /^FAIL$/ { bad=1 }
  END { exit bad || root != 1 || complete != 1 || seen["stop"] != 1 || seen["handoff"] != 1 || seen["before"] != 1 || seen["after"] != 1 }' "$mode/guest.log" || return 1
  client_verify_dns "$mode/guest.log" 8 1 4 0 || return 1
}
