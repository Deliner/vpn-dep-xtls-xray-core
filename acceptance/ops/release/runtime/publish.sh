#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 ]] || exit 2
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
cd "$root"
stage="$(cd "$1" && pwd -P)"
case "$stage/" in "$root/build/runtime-releases/"*) ;; *) exit 2 ;; esac
go run ./cmd/runtimepack verify ops/release/runtime/xray.json "$stage"
[[ ! -L "$root/build" && ! -L "$root/build/runtime-releases" ]] || exit 2
work="$(mktemp -d "$root/build/runtime-releases/.publish-XXXXXXXX")"
mkdir "$work/upload" "$work/download"
names=(Xray-linux-amd64.zip Xray-linux-arm64.zip xray-amd64.json xray-arm64.json SHA256SUMS notes.md tag target)
assets=()
for name in "${names[@]}"; do cp "$stage/$name" "$work/upload/$name"; assets+=("$work/upload/$name"); done
go run ./cmd/runtimepack verify ops/release/runtime/xray.json "$work/upload"
repo=Deliner/vpn-dep-xtls-xray-core
tag="$(cat "$work/upload/tag")"
target="$(cat "$work/upload/target")"
[[ "$tag" =~ ^nhop-xray-[0-9a-z.-]+$ && "$target" =~ ^[0-9a-f]{40}$ ]] || exit 2
[[ "$(gh api --hostname github.com user --jq '[.login,.id]|@tsv')" == $'Deliner\t26311467' ]] || exit 2
[[ "$(gh api --hostname github.com "repos/$repo" --jq '[.id,.owner.id,.full_name,.private,.fork]|@tsv')" == $'1449369505\t26311467\tDeliner/vpn-dep-xtls-xray-core\tfalse\ttrue' ]] || exit 2
[[ "$(gh api --hostname github.com "repos/$repo/actions/permissions" --jq .enabled)" == false ]] || { printf 'Publication requires the owned fork Actions policy to remain disabled.\n' >&2; exit 2; }
[[ "$(gh api --hostname github.com "repos/$repo/commits/$target" --jq .sha)" == "$target" ]] || exit 2
gh api --hostname github.com --paginate "repos/$repo/releases?per_page=100" --jq '.[]|[.tag_name,.id]|@tsv' > "$work/releases-before.tsv"
existing="$(awk -F '\t' -v tag="$tag" '$1 == tag {print $2}' "$work/releases-before.tsv")"
if [[ -z "$existing" ]]; then
  gh release create "$tag" --repo "$repo" --target "$target" --draft --prerelease --latest=false \
    --title "$tag" --notes-file "$work/upload/notes.md" "${assets[@]}" > "$work/create.log" 2>&1
else
  [[ "$existing" =~ ^[0-9]+$ ]] || exit 2
fi
gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isDraft,isPrerelease,databaseId,url > "$work/release-before.json"
[[ "$(gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isPrerelease --jq '[.tagName,.targetCommitish,.isPrerelease]|@tsv')" == "$tag"$'\t'"$target"$'\ttrue' ]] || exit 2
gh release download "$tag" --repo "$repo" --dir "$work/download"
go run ./cmd/runtimepack verify ops/release/runtime/xray.json "$work/download"
for name in "${names[@]}"; do cmp "$work/upload/$name" "$work/download/$name"; done
if [[ "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" == true ]]; then
  gh release edit "$tag" --repo "$repo" --draft=false --prerelease --latest=false
fi
[[ "$(gh api --hostname github.com "repos/$repo/git/ref/tags/$tag" --jq .object.sha)" == "$target" ]] || exit 2
[[ "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" == false ]] || exit 2
gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isDraft,isPrerelease,databaseId,url > "$work/release-after.json"
printf 'Published and downloaded bytes verified: https://github.com/%s/releases/tag/%s\nEvidence: %s\n' "$repo" "$tag" "$work"
