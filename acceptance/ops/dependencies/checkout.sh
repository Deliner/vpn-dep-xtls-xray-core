#!/usr/bin/env bash
set -euo pipefail
umask 077

repository="${1:-}"
revision="${2:-}"
mode="${3:-full}"
[[ "$repository" =~ ^Deliner/vpn-dep-[a-z0-9._-]+$ ]] || { printf 'Expected a maintained Deliner dependency fork\n' >&2; exit 2; }
[[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { printf 'Checkout requires a full reviewed commit SHA\n' >&2; exit 2; }
case "$mode" in
  full) ;;
  cronet-source) [[ "$repository" == Deliner/vpn-dep-sagernet-cronet-go ]] || exit 2 ;;
  *) printf 'Unsupported checkout mode\n' >&2; exit 2 ;;
esac
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
parent="$root/build/dependencies/checkouts"
for directory in "$root/build" "$root/build/dependencies" "$parent"; do
  [[ ! -L "$directory" ]] || { printf 'Symlink output parent refused\n' >&2; exit 2; }
  mkdir -p "$directory"
done
target="$parent/${repository#*/}"
[[ ! -e "$target" && ! -L "$target" ]] || { printf 'Checkout already exists: %s\n' "$target" >&2; exit 2; }

# Fetch only the selected source revision; never change an existing checkout.
export GIT_TERMINAL_PROMPT=0 GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
git init --initial-branch=nhop-maintained "$target"
git -C "$target" remote add origin "https://github.com/$repository.git"
if [[ "$mode" == cronet-source ]]; then
  # Preserve the exact Git tree while avoiding unrelated prebuilt native blobs.
  # Sparse paths remain tracked; this is not a complete source archive export.
  git -C "$target" config remote.origin.promisor true
  git -C "$target" config remote.origin.partialclonefilter blob:none
  git -C "$target" -c protocol.file.allow=never fetch --filter=blob:none --depth=1 origin "$revision"
  git -C "$target" sparse-checkout set --no-cone '/*' '!/lib/' '!/all/'
else
  git -C "$target" -c protocol.file.allow=never fetch --depth=1 origin "$revision"
fi
git -C "$target" checkout -B nhop-maintained FETCH_HEAD
[[ "$(git -C "$target" rev-parse HEAD)" == "$revision" ]]
printf 'Checkout: %s\nRevision: %s\n' "$target" "$revision"
