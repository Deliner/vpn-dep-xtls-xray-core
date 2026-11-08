#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/identity.sh"
[[ $# == 1 ]] || exit 2
acceptance_identity
acceptance_directory "$1"
[[ "$(cat "$acceptance_output/status")" == prepared ]] || { printf 'This preparation has already been published or attempted; inspect its state.\n' >&2; exit 2; }
[[ "$(git -C "$acceptance_root" rev-parse HEAD)" == "$(cat "$acceptance_output/project.commit")" ]] || exit 2
acceptance_capture_actions
gh api --hostname github.com --paginate "repos/$acceptance_repo/actions/workflows?per_page=100" \
  --jq '.workflows[]|[.id,.state,.path]|@tsv' > "$acceptance_output/workflows-before.tsv"
printf 'disabling-inherited-workflows\n' > "$acceptance_output/status"
prepared_workflow=
while IFS=$'\t' read -r workflow_id workflow_state workflow_path; do
  [[ "$workflow_id" =~ ^[0-9]+$ ]] || exit 2
  if [[ "$workflow_state" == active ]]; then
    gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/workflows/$workflow_id/disable"
  fi
  if [[ "$workflow_path" == .github/workflows/nhop-acceptance.yml ]]; then prepared_workflow="$workflow_id"; fi
done < "$acceptance_output/workflows-before.tsv"
gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/permissions" -F enabled=true
acceptance_verify_enabled_policy
if [[ -n "$prepared_workflow" ]]; then
  gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/workflows/$prepared_workflow/enable"
fi
printf 'enabled-for-acceptance\n' > "$acceptance_output/actions-state"
printf 'push-in-progress\n' > "$acceptance_output/status"
GIT_TERMINAL_PROMPT=0 git -C "$acceptance_checkout" -c credential.helper= \
  -c 'credential.helper=!gh auth git-credential' push --porcelain origin HEAD:refs/heads/nhop-acceptance
[[ "$(gh api --hostname github.com "repos/$acceptance_repo/git/ref/heads/nhop-acceptance" --jq .object.sha)" == "$acceptance_commit" ]] || exit 2
printf 'published\n' > "$acceptance_output/status"
printf 'Published acceptance SHA: %s\n' "$acceptance_commit"
