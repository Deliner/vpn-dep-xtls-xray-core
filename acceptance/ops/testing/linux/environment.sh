#!/usr/bin/env bash
# This checkout owns one VM. Lima must keep its atomic known_hosts beside its SSH metadata.
lab_environment() {
  lab_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
  [[ "$(uname -s)" == Darwin ]] || { printf 'The private VZ lab requires macOS.\n' >&2; return 2; }
  case "$(uname -m)" in
    arm64) lab_arch=arm64; lab_guest_arch=aarch64 ;;
    x86_64) lab_arch=amd64; lab_guest_arch=x86_64 ;;
    *) printf 'Unsupported lab CPU.\n' >&2; return 2 ;;
  esac
  lab_data="$lab_root/build/lab"
  lab_id="$(printf '%s' "$lab_root" | shasum -a 256 | cut -c 1-12)"
  export LIMA_HOME="$HOME/.ssh-custom/nhop-lab-$lab_id"
  lab_instance=payload
  lab_lima="$lab_data/tools/bin/limactl"
}

lab_owned() {
  [[ -f "$LIMA_HOME/.nhop-owner" && ! -L "$LIMA_HOME" ]] || { printf 'Run just lab-setup first.\n' >&2; return 2; }
  [[ "$(cat "$LIMA_HOME/.nhop-owner")" == "$lab_root" ]] || { printf 'Lab owner differs from this checkout.\n' >&2; return 2; }
  [[ -x "$lab_lima" ]] || { printf 'Missing private Lima tool; run just lab-setup.\n' >&2; return 2; }
}

lab_shell() {
  "$lab_lima" shell --workdir /home/nhop "$lab_instance" "$@"
}
