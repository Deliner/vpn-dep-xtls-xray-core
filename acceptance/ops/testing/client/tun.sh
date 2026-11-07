#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../../runtime/integrity.sh"
client_lab_begin
for directory in "$1" "$2"; do
  candidate_complete "$directory"
  [[ "$(sed -n 's/^target=//p' "$directory/provenance.txt")" == "linux/$lab_arch" ]] || exit 2
  candidate_inventory "$directory" > "$run/observed.sha256"
  cmp "$run/observed.sha256" "$directory/SHA256SUMS"
done
rm "$run/observed.sha256"
cp "$1/bin/xray" "$2/bin/sing-box" "$2/bin/libcronet.so" "$run/bin/"
client_lab_run tun
