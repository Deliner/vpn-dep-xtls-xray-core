#!/usr/bin/env bash
# Evidence contracts, sourced by collect.sh and exercised with damaged fixtures.
client_verify_build() {
  local binary="$1" observed="$2" arch="$3"
  go version -m "$binary" > "$observed" || return 1
  grep -q -- '-race=true' "$observed" || return 1
  grep -Fq $'\tbuild\tCGO_ENABLED=1' "$observed" || return 1
  grep -Fq $'\tbuild\tGOOS=linux' "$observed" || return 1
  grep -Fq $'\tbuild\tGOARCH='"$arch" "$observed" || return 1
}

client_verify_public() {
  local mode="$1" policy="$2" failure expected
  grep -q "client policy: IPv6=$policy" "$mode/guest.log" || return 1
  for failure in graceful native controller-term controller-kill reconnect-native reconnect-frontend resume-term resume-kill \
    intent-before-resume intent-after-resume intent-before-stop intent-after-stop; do
    expected="--- PASS: TestKernelPublicClientAndControllerRecovery/$failure "
    awk -v expected="$expected" 'index($0, expected) { count++ } END { exit count != 1 }' "$mode/guest.log" || return 1
  done
  for phase in direct tunnel blocked; do grep -q "system DNS $phase:" "$mode/guest.log" || return 1; done
  awk '/reconnect direct UDP: attempts=[0-9]+ delivered=0; same firewall lease retained/ { count++ } END { exit count != 2 }' "$mode/guest.log" || return 1
  awk '/resume direct UDP: attempts=[0-9]+ delivered=0; same firewall lease retained/ { count++ } END { exit count != 2 }' "$mode/guest.log" || return 1
  awk '/^--- PASS: TestKernelPublicClientAndControllerRecovery \(/ { root++ }
    /^PASS$/ { complete++ } /--- (FAIL|SKIP):/ || /^FAIL$/ { bad=1 }
    END { exit bad || root != 1 || complete != 1 }' "$mode/guest.log" || return 1
  client_verify_dns "$mode/guest.log" 26 18 13 0 || return 1
  client_verify_intents "$mode/guest.log" || return 1
}

client_verify_intents() {
  awk '
  /planned guard crash:/ {
    value=$0; sub(/^.*planned guard crash: /,"",value); if (split(value,f,/ +/)!=4) bad=1
    stage=f[1]; sub(/^stage=/,"",stage); token=f[2]; sub(/^token=/,"",token)
    handle=f[4]; sub(/^observed_handle=/,"",handle)
    if (f[1] !~ /^stage=(before|after)$/ || f[2] !~ /^token=nhop:[0-9a-f]+$/ || length(token)!=37 || f[3]!="handle=0" || f[4] !~ /^observed_handle=[0-9]+$/) bad=1
    if (seen[token]++ || (stage=="before" && handle+0!=0) || (stage=="after" && handle+0==0)) bad=1
    tokens[stage]=token; handles[stage]=handle+0; crashes[stage]++
  }
  /planned guard resumed:/ {
    value=$0; sub(/^.*planned guard resumed: /,"",value); if (split(value,f,/ +/)!=5) bad=1
    stage=f[1]; sub(/^stage=/,"",stage); token=f[2]; sub(/^token=/,"",token)
    handle=f[3]; sub(/^handle=/,"",handle); attempts=f[4]; sub(/^attempts=/,"",attempts)
    if (f[1] !~ /^stage=(before|after)$/ || f[2] !~ /^token=nhop:[0-9a-f]+$/ || token!=tokens[stage] || f[3] !~ /^handle=[0-9]+$/ || handle+0==0 || f[4] !~ /^attempts=[0-9]+$/ || attempts+0<20 || f[5]!="delivered=0") bad=1
    if (stage=="after" && handle+0!=handles[stage]) bad=1
    resumed[stage]++
  }
  /planned guard recovery verified:/ {
    value=$0; sub(/^.*planned guard recovery verified: /,"",value); verified[value]++
    if (value !~ /^stage=(before|after) action=(resume|stop)$/) bad=1
  }
  END {
    if (crashes["before"]!=2 || crashes["after"]!=2 || resumed["before"]!=1 || resumed["after"]!=1) bad=1
    if (verified["stage=before action=resume"]!=1 || verified["stage=after action=resume"]!=1 || verified["stage=before action=stop"]!=1 || verified["stage=after action=stop"]!=1) bad=1
    exit bad
  }' "$1" || return 1
}

client_verify_systemd() {
  client_verify_recovery "$1" "$2" TestKernelSystemdManagedRecovery || return 1
  client_verify_service_unit "$1" "$2" '^/run/systemd/system/nhop-client-acceptance-[0-9]+\.service$' || return 1
}

client_verify_recovery() {
  local mode="$1" policy="$2" test_name="$3" failure
  grep -q "systemd policy IPv6=$policy: systemd " "$mode/guest.log" || return 1
  awk -v name="$test_name" 'index($0,"--- PASS: " name " (") { root++ }
    /^PASS$/ { complete++ } /--- (FAIL|SKIP):/ || /^FAIL$/ { bad=1 }
    END { exit bad || root != 1 || complete != 1 }' "$mode/guest.log" || return 1
  for failure in native frontend controller restart; do
    awk -v expected="systemd recovered $failure;" 'index($0, expected) { count++ } END { exit count != 1 }' "$mode/guest.log" || return 1
  done
  awk '/systemd direct UDP: attempts=[0-9]+ delivered=0; same firewall lease retained/ {
    value=$0; sub(/^.*attempts=/,"",value); if (value+0 < 20) bad=1; count++
  } END { exit bad || count != 1 }' "$mode/guest.log" || return 1
  client_verify_dns "$mode/guest.log" 2 6 1 1 1 || return 1
}

client_verify_service_unit() {
  local mode="$1" policy="$2" target_pattern="$3" digest target
  [[ "$(wc -l < "$mode/systemd-unit.sha256" | tr -d ' ')" == 1 ]] || return 1
  read -r digest target < "$mode/systemd-unit.sha256" || return 1
  [[ "$digest" =~ ^[0-9a-f]{64}$ && "$target" =~ $target_pattern ]] || return 1
  [[ "$(shasum -a 256 "$mode/systemd-unit.txt" | awk '{ print $1 }')" == "$digest" ]] || return 1
  grep -q '^ExecStart=:' "$mode/systemd-unit.txt" || return 1
  grep -q '^ExecStartPre=:' "$mode/systemd-unit.txt" || return 1
  grep -Fxq 'ExecPaths=-/run/nhop-client' "$mode/systemd-unit.txt" || return 1
  grep -Fq '"--ipv6" "'"$policy"'"' "$mode/systemd-unit.txt" || return 1
  awk '/^(ExecStop=|RuntimeDirectory=|\[Install\])/ { bad=1 } END { exit bad }' "$mode/systemd-unit.txt" || return 1
}

client_verify_dns() {
  awk -v direct_count="${2:-2}" -v tunnel_count="${3:-6}" -v blocked_count="${4:-1}" -v contiguous="${5:-1}" -v disconnect_count="${6:-0}" '
  /system DNS disconnect requested:/ {
    value=$0; sub(/^.*disconnect requested: /,"",value); split(value,fields,/ +/)
    if (fields[1] !~ /^direct=[0-9]+$/ || fields[2] !~ /^HTTPS=[0-9]+$/) bad=1
    sub(/^direct=/,"",fields[1]); sub(/^HTTPS=/,"",fields[2]); disconnects++
    if (!seen || counts["blocked"] != blocked_count || fields[1]+0 != previous_direct || fields[2]+0 != previous_https) bad=1
    disconnected=1
  }
  /system DNS (direct|tunnel|blocked):/ {
    value=$0; sub(/^.*system DNS /,"",value); split(value,fields,/[: =]+/)
    kind=fields[1]; split(fields[3],direct,"->"); split(fields[5],https,"->")
    if (fields[3] !~ /^[0-9]+->[0-9]+$/ || fields[5] !~ /^[0-9]+->[0-9]+$/) bad=1
    if (disconnected && (kind!="direct" || https[1]+0 != previous_https)) bad=1
    if (contiguous && seen && !disconnected && (direct[1]+0 != previous_direct || https[1]+0 != previous_https)) bad=1
    if (seen && (direct[1]+0 < previous_direct || https[1]+0 < previous_https)) bad=1
    if (kind=="direct" && !(direct[2]+0 > direct[1]+0 && https[1]==https[2])) bad=1
    if (kind=="tunnel" && !(https[2]+0 > https[1]+0 && direct[1]==direct[2])) bad=1
    if (kind=="blocked" && !(direct[1]==direct[2] && https[1]==https[2])) bad=1
    seen=1; disconnected=0; previous_direct=direct[2]+0; previous_https=https[2]+0; counts[kind]++
  } END { exit bad || disconnected || disconnects != disconnect_count || counts["direct"] != direct_count || counts["tunnel"] != tunnel_count || counts["blocked"] != blocked_count }' "$1" || return 1
}
