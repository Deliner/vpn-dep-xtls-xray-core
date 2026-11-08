#!/usr/bin/env bash
# Frozen scripts and executable identities for one completed fixture.
client_verify_manifest() {
  local directory="$1" listing="$2"
  find "$directory" -type l -print > "$listing.links" || return 1
  [[ ! -s "$listing.links" ]] || return 1
  awk 'length($1) != 64 || $1 ~ /[^0-9a-f]/ || NF != 2 || $2 !~ /^\.\// || $2 ~ /(^|\/)\.\.(\/|$)/ { bad=1 }
    END { exit bad }' "$directory/evidence.sha256" || return 1
  (cd "$directory"; find . -type f ! -name evidence.sha256 -print | LC_ALL=C sort) > "$listing.observed" || return 1
  awk '{ print $2 }' "$directory/evidence.sha256" | LC_ALL=C sort > "$listing.expected" || return 1
  cmp "$listing.expected" "$listing.observed" || return 1
}

client_verify_inputs() {
  local kind="$1" mode="$2" name engine digest
  local names=(network.test guest.sh resolved-guest.sh bin/xray bin/sing-box bin/libcronet.so)
  if [[ "$kind" == systemd || "$kind" == installed ]]; then names+=(systemd-guest.sh); fi
  if [[ "$kind" == installed ]]; then names+=(installed-guest.sh); fi
  if [[ "$kind" != public ]]; then names+=(bin/nhop-client); fi
  [[ "$(cat "$mode/status")" == passed ]] || return 1
  for name in "${names[@]}"; do
    [[ -f "$mode/$name" && ! -L "$mode/$name" && -s "$mode/$name" ]] || return 1
    awk -v name="$name" '$2 == name { found++ } END { exit found != 1 }' "$mode/inputs.sha256" || return 1
  done
  [[ "$(wc -l < "$mode/inputs.sha256" | tr -d ' ')" == "${#names[@]}" ]] || return 1
  (cd "$mode"; shasum -a 256 --check inputs.sha256) >> "$report/integrity-verification.log" || return 1
  cmp "$mode/network.test" "$report/network.test" || return 1
  cmp "$mode/guest.sh" "$source_root/ops/testing/client/kernel-guest.sh" || return 1
  cmp "$mode/resolved-guest.sh" "$source_root/ops/testing/client/resolved-guest.sh" || return 1
  if [[ "$kind" == systemd || "$kind" == installed ]]; then
    cmp "$mode/systemd-guest.sh" "$source_root/ops/testing/client/systemd-guest.sh" || return 1
  fi
  if [[ "$kind" == installed ]]; then
    cmp "$mode/installed-guest.sh" "$source_root/ops/testing/client/install/managed-guest.sh" || return 1
  fi
  if [[ "$kind" != public ]]; then
    cmp "$mode/bin/nhop-client" "$report/nhop-client" || return 1
  fi
  for name in xray sing-box libcronet.so; do
    engine=sing-box
    if [[ "$name" == xray ]]; then engine=xray; fi
    digest="$(awk -v name="bin/$name" '$2 == name { print $1 }' "$report/$engine-SHA256SUMS")"
    [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
    printf '%s  %s\n' "$digest" "$mode/bin/$name" | shasum -a 256 --check >> "$report/integrity-verification.log" || return 1
  done
  [[ -s "$mode/host-before.txt" && -s "$mode/host-after.txt" ]] || return 1
  cmp "$mode/host-before.txt" "$mode/host-after.txt" || return 1
  grep -q "native client acceptance: linux/$arch; private mount and network namespaces" "$mode/guest.log" || return 1
  awk '/WARNING: DATA RACE/ { bad=1 } END { exit bad }' "$mode/guest.log" || return 1
}
