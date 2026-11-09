#!/usr/bin/env bash
# A reviewed project snapshot supplies the actual previous installer on both CPUs.
installer_previous_pin() {
  local root="$1" count
  count="$(sed '/^#/d; /^$/d' "$root/ops/testing/acceptance/installer/previous.tsv" | wc -l | tr -d ' ')"
  [[ "$count" == 1 ]] || return 2
  IFS=$'\t' read -r previous_commit previous_sha previous_bytes < <(sed '/^#/d; /^$/d' "$root/ops/testing/acceptance/installer/previous.tsv")
  [[ "$previous_commit" =~ ^[0-9a-f]{40}$ && "$previous_sha" =~ ^[0-9a-f]{64}$ && "$previous_bytes" =~ ^[1-9][0-9]*$ ]] || return 2
}

installer_previous_archive() {
  local root="$1" target="$2"
  installer_previous_pin "$root" || return
  [[ ! -e "$target" && ! -L "$target" ]] || return 2
  if [[ -e "$root/previous-source.tar" || -L "$root/previous-source.tar" ]]; then
    [[ -f "$root/previous-source.tar" && ! -L "$root/previous-source.tar" ]] || return 2
    cp "$root/previous-source.tar" "$target" || return
  else
    git -C "$root" archive --format=tar "$previous_commit" go.mod go.sum justfile cmd internal ops \
      examples/nhop/topology.yaml > "$target" || return
  fi
  [[ "$(wc -c < "$target" | tr -d ' ')" == "$previous_bytes" ]] || return 2
  [[ "$(shasum -a 256 "$target" | awk '{print $1}')" == "$previous_sha" ]] || return 2
}
