#!/usr/bin/env bash
set -euo pipefail
umask 077

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
output="${1:-$repo_root/build/vpn-script-public.tar.gz}"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

case "$output" in
  *.tar.gz) ;;
  *) printf 'error: output must be a .tar.gz archive\n' >&2; exit 1 ;;
esac
root="$stage/vpn-script"
install -d -m 0755 "$root"
files=(.gitignore AGENTS.md README.md go.mod go.sum justfile .github/workflows/go.yml)

while IFS= read -r -d '' source; do
  files+=("${source#"$repo_root/"}")
# Embedded HTML/CSS are executable build inputs, just like their Go owners.
done < <(find "$repo_root/cmd" "$repo_root/internal" -type f \( -name '*.go' -o -name '*.html' -o -name '*.css' \) -print0)
while IFS= read -r -d '' source; do
  files+=("${source#"$repo_root/"}")
done < <(find "$repo_root/docs" -type f -name '*.md' ! -name '*.local.md' -print0)
while IFS= read -r -d '' source; do
  files+=("${source#"$repo_root/"}")
done < <(find "$repo_root/ops" -type f \( -name '*.sh' -o -name '*.md' -o -name '*.just' -o -name '*.tsv' -o -name '*.patch' \) ! -name '*.local.md' -print0)
# Candidate module locks are maintained inputs, not downloaded upstream source.
while IFS= read -r -d '' source; do
  files+=("${source#"$repo_root/"}")
done < <(find "$repo_root/ops/runtime" -type f \( -name go.mod -o -name go.sum \) -print0)
while IFS= read -r -d '' source; do
  files+=("${source#"$repo_root/"}")
done < <(find "$repo_root/examples" -type f \( -name '*.yaml' -o -name '*.md' -o -name 'config.env.example' \) -print0)

for relative in "${files[@]}"; do
  install -d -m 0755 "$(dirname "$root/$relative")"
  install -m 0644 "$repo_root/$relative" "$root/$relative"
done
find "$root/ops" -type f -name '*.sh' -exec chmod 0755 {} +
if command -v xattr >/dev/null 2>&1; then xattr -cr "$root"; fi
(
  cd "$stage"
  find vpn-script -type f ! -name SHA256SUMS -print0 | sort -z | \
    xargs -0 shasum -a 256 > vpn-script/ops/release/SHA256SUMS
  chmod 0644 vpn-script/ops/release/SHA256SUMS
)
mkdir -p "$(dirname "$output")"
if tar --version 2>/dev/null | head -n 1 | grep -q 'GNU tar'; then
  owner_flags=(--owner=0 --group=0 --numeric-owner)
else
  owner_flags=(--uid 0 --gid 0 --numeric-owner)
fi
COPYFILE_DISABLE=1 tar "${owner_flags[@]}" -czf "$output" -C "$stage" vpn-script
chmod 0600 "$output"
printf 'Created source export: %s\n' "$output"
printf 'Excluded private/generated/history material: config.env, build, .git, local metadata.\n'
