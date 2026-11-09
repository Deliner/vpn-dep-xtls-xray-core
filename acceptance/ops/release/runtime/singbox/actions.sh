#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ $# == 1 ]] || exit 2
repo=Deliner/vpn-dep-sagernet-sing-box
[[ "$(gh api --hostname github.com user --jq '[.login,.id]|@tsv')" == $'Deliner\t26311467' ]] || exit 2
[[ "$(gh api --hostname github.com "repos/$repo" --jq '[.id,.owner.id,.full_name,.private,.fork]|@tsv')" == $'1449371112\t26311467\tDeliner/vpn-dep-sagernet-sing-box\tfalse\ttrue' ]] || exit 2
bash ops/release/runtime/singbox/verify.sh "$1"
[[ ! -L build && ! -L build/runtime-releases ]] || exit 2
work="$(mktemp -d "$PWD/build/runtime-releases/.singbox-actions-XXXXXXXX")"
gh api --hostname github.com "repos/$repo/actions/permissions" > "$work/before.json"
policy="$(gh api --hostname github.com "repos/$repo/actions/permissions" --jq '[.enabled,(.allowed_actions // "absent"),.sha_pinning_required]|@tsv')"
case "$policy" in $'true\tall\tfalse'|$'false\tabsent\tfalse') ;; *) printf 'Unreviewed Actions policy; no publication attempted.\n' >&2; exit 2 ;; esac
gh api --hostname github.com --paginate "repos/$repo/actions/runs?per_page=100" --jq '.workflow_runs[]|select(.status!="completed")|[.id,.status]|@tsv' > "$work/live-runs.tsv"
[[ ! -s "$work/live-runs.tsv" ]] || { printf 'Owned fork has live runs; preserve them.\n' >&2; exit 2; }
gh api --hostname github.com --paginate "repos/$repo/actions/workflows?per_page=100" --jq '.workflows[]|[.id,.state,.path]|@tsv' > "$work/workflows-before.tsv"
restore=0
verify_workflows() {
  local workflow_id workflow_state workflow_path actual
  gh api --hostname github.com --paginate "repos/$repo/actions/workflows?per_page=100" --jq '.workflows[]|[.id,.state,.path]|@tsv' > "$work/workflows-after.tsv"
  while IFS=$'\t' read -r workflow_id workflow_state workflow_path; do
    actual="$(awk -F '\t' -v id="$workflow_id" '$1 == id {print $2 "\t" $3}' "$work/workflows-after.tsv")"
    [[ "$actual" == "$workflow_state"$'\t'"$workflow_path" ]] || return 2
  done < "$work/workflows-before.tsv"
  # GitHub can register existing source workflows lazily after the first release.
  # Record these metadata additions; never disable or modify their definitions.
  awk -F '\t' 'FILENAME == ARGV[1] {seen[$1]=1; next} !seen[$1]' "$work/workflows-before.tsv" "$work/workflows-after.tsv" > "$work/registered-workflows.tsv"
}
restore_actions() {
  local result=$?
  if ((restore)); then
    if gh api --hostname github.com --method PUT "repos/$repo/actions/permissions" --input "$work/before.json" > "$work/restore.log" 2>&1 &&
       gh api --hostname github.com "repos/$repo/actions/permissions" > "$work/after.json" && cmp "$work/before.json" "$work/after.json" && verify_workflows; then
      printf 'Original sing-box Actions policy restored. Evidence: %s\n' "$work"
    else
      printf 'Actions restoration requires attention; saved original policy: %s/before.json\n' "$work" >&2
      result=1
    fi
  fi
  exit "$result"
}
trap restore_actions EXIT
if [[ "$policy" == $'true\tall\tfalse' ]]; then
  restore=1
  gh api --hostname github.com --method PUT "repos/$repo/actions/permissions" -F enabled=false
fi
bash ops/release/runtime/singbox/publish.sh "$1"
verify_workflows
