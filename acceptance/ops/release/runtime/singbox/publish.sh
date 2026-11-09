#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 ]] || exit 2
root="$PWD"
stage="$(cd "$1" && pwd -P)"
case "$stage/" in "$root/build/runtime-releases/"*) ;; *) exit 2 ;; esac
[[ ! -L "$root/build" && ! -L "$root/build/runtime-releases" ]] || exit 2
bash ops/release/runtime/singbox/verify.sh "$stage"
work="$(mktemp -d "$root/build/runtime-releases/.singbox-publish-XXXXXXXX")"
mkdir "$work/upload" "$work/download"
cp "$stage/"* "$work/upload/"
bash ops/release/runtime/singbox/verify.sh "$work/upload"
repo=Deliner/vpn-dep-sagernet-sing-box
tag="$(awk -F '\t' '$1 == "tag" {print $2}' "$work/upload/publication.tsv")"
target="$(awk -F '\t' '$1 == "target" {print $2}' "$work/upload/publication.tsv")"
[[ "$tag" =~ ^nhop-sing-box-[0-9a-z.-]+$ && "$target" =~ ^[0-9a-f]{40}$ ]] || exit 2
[[ "$(gh api --hostname github.com user --jq '[.login,.id]|@tsv')" == $'Deliner\t26311467' ]] || exit 2
[[ "$(gh api --hostname github.com "repos/$repo" --jq '[.id,.owner.id,.full_name,.private,.fork]|@tsv')" == $'1449371112\t26311467\tDeliner/vpn-dep-sagernet-sing-box\tfalse\ttrue' ]] || exit 2
gh api --hostname github.com "repos/$repo/actions/permissions" > "$work/actions-before.json"
[[ "$(gh api --hostname github.com "repos/$repo/actions/permissions" --jq .enabled)" == false ]] || exit 2
[[ "$(gh api --hostname github.com "repos/$repo/commits/$target" --jq .sha)" == "$target" ]] || exit 2
gh api --hostname github.com --paginate "repos/$repo/git/matching-refs/tags/$tag" --jq '.[]|[.ref,.object.sha]|@tsv' > "$work/refs-before.tsv"
ref="$(awk -F '\t' -v ref="refs/tags/$tag" '$1 == ref {print $2}' "$work/refs-before.tsv")"
[[ -z "$ref" || "$ref" == "$target" ]] || exit 2
gh api --hostname github.com --paginate "repos/$repo/releases?per_page=100" --jq '.[]|[.tag_name,.id]|@tsv' > "$work/releases-before.tsv"
existing="$(awk -F '\t' -v tag="$tag" '$1 == tag {print $2}' "$work/releases-before.tsv")"
if [[ -z "$existing" ]]; then
  gh release create "$tag" --repo "$repo" --target "$target" --draft --prerelease --latest=false \
    --title "$tag" --notes-file "$work/upload/notes.md" "$work/upload/"* > "$work/create.log" 2>&1
else
  [[ "$existing" =~ ^[0-9]+$ ]] || exit 2
fi
gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isDraft,isPrerelease,databaseId,url > "$work/release-before.json"
[[ "$(gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isPrerelease --jq '[.tagName,.targetCommitish,.isPrerelease]|@tsv')" == "$tag"$'\t'"$target"$'\ttrue' ]] || exit 2
gh release download "$tag" --repo "$repo" --dir "$work/download"
bash ops/release/runtime/singbox/verify.sh "$work/download"
for asset in "$work/upload/"*; do cmp "$asset" "$work/download/${asset##*/}"; done
if [[ "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" == true ]]; then
  gh release edit "$tag" --repo "$repo" --draft=false --prerelease --latest=false
fi
[[ "$(gh api --hostname github.com "repos/$repo/git/ref/tags/$tag" --jq .object.sha)" == "$target" ]] || exit 2
[[ "$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft)" == false ]] || exit 2
gh api --hostname github.com "repos/$repo/actions/permissions" > "$work/actions-after.json"
cmp "$work/actions-before.json" "$work/actions-after.json"
gh release view "$tag" --repo "$repo" --json tagName,targetCommitish,isDraft,isPrerelease,databaseId,url > "$work/release-after.json"
printf 'Published verified sing-box prerelease: https://github.com/%s/releases/tag/%s\nEvidence: %s\n' "$repo" "$tag" "$work"
