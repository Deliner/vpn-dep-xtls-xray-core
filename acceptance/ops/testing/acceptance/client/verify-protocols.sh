#!/usr/bin/env bash
# The independent acceptance contract is checked against the Go catalog by tests.
client_verify_protocols() {
  local mode="$1" policy="$2"
  local names='anytls hysteria2 hysteria2-gecko hysteria2-salamander masque-h2 masque-h3 naive-h2 naive-h3 shadowsocks shadowsocks-2022 shadowtls-v3 snell-v5 snell-v6 trojan trojan-ws tuic vless-encryption vless-reality vless-tls vless-ws vless-xhttp vmess-tls vmess-ws'
  grep -Fq "TUN matrix policy: IPv6=$policy; real CLI import/start/reconnect/stop" "$mode/guest.log" || return 1
  awk -v names="$names" -v policy="$policy" '
    BEGIN { total=split(names,catalog," "); for (i=1;i<=total;i++) position[catalog[i]]=i }
    /TUN route=/ {
      value=$0; sub(/^.*TUN route=/,"",value); split(value,fields," "); name=fields[1]
      if (!(name in position) || fields[2] !~ /^engine=(xray|sing-box)$/) bad=1
      sub(/^.*path=\[/,"",value); sub(/\]$/,"",value); hops=split(value,path," ")
      if (hops!=1 && hops!=3) bad=1
      for (j=1;j<=hops;j++) if (path[j]!="node" (j-1) "." catalog[(position[name]+j-2)%total+1]) bad=1
      routes[hops "/" name]++
    }
    /TUN protocol=/ {
      value=$0; sub(/^.*TUN protocol=/,"",value); split(value,fields," ")
      name=fields[1]; hops=fields[2]; sub(/^hops=/,"",hops)
      if (!(name in position) || (hops!=1 && hops!=3) || fields[3]!="IPv6=" policy) bad=1
      if (fields[6] !~ /^attempts=[0-9]+$/ || fields[7]!="delivered=0;") bad=1
      sub(/^attempts=/,"",fields[6]); if (fields[6]+0 < 20) bad=1
      if (value !~ /; same firewall lease retained$/) bad=1
      canaries[hops "/" name]++
    }
    /--- PASS: TestKernelAllProtocolTUN\/[13]-hop\// {
      value=$0; sub(/^.*TestKernelAllProtocolTUN\//,"",value); sub(/ \(.*/,"",value)
      sub(/-hop\//,"/",value); passed[value]++
    }
    /--- PASS: TestKernelAllProtocolTUN \(/ { root++ }
    /^PASS$/ { complete++ }
    /--- (FAIL|SKIP): TestKernelAllProtocolTUN/ || /^FAIL$/ { bad=1 }
    END {
      for (h=1;h<=3;h+=2) for(i=1;i<=total;i++) {
        key=h "/" catalog[i]; if (routes[key]!=1 || canaries[key]!=1 || passed[key]!=1) bad=1
      }
      for (key in routes) if (!(key in passed)) bad=1
      for (key in canaries) if (!(key in passed)) bad=1
      if (length(routes)!=46 || length(canaries)!=46 || length(passed)!=46) bad=1
      exit bad || root!=1 || complete!=1
    }' "$mode/guest.log" || return 1
  # Separate sessions have ordinary networking before start and after explicit stop;
  # each controlled DNS lookup must still prove its own route or complete denial.
  client_verify_dns "$mode/guest.log" 92 92 46 0 || return 1
}
