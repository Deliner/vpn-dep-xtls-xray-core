#!/usr/bin/env bash
# Identity and immutable inputs shared by local CI preparation/publication/collection.
acceptance_identity() {
  acceptance_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
  acceptance_repo=Deliner/vpn-dep-xtls-xray-core
  export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_TERMINAL_PROMPT=0
  [[ "$(gh api --hostname github.com user --jq '[.login,.id]|@tsv')" == $'Deliner\t26311467' ]] || return 2
  [[ "$(gh api --hostname github.com "repos/$acceptance_repo" --jq '[.id,.owner.id,.full_name,.private,.fork]|@tsv')" == $'1449369505\t26311467\tDeliner/vpn-dep-xtls-xray-core\tfalse\ttrue' ]] || return 2
}

acceptance_directory() {
  acceptance_output="$(cd "$1" && pwd -P)"
  case "$acceptance_output/" in "$acceptance_root/build/acceptance/"*) ;; *) return 2 ;; esac
  [[ "$(cat "$acceptance_output/repository-name")" == "$acceptance_repo" ]] || return 2
  acceptance_checkout="$acceptance_output/repository"
  acceptance_commit="$(cat "$acceptance_output/prepared.commit")"
  [[ "$acceptance_commit" =~ ^[0-9a-f]{40}$ ]] || return 2
  [[ "$(git -C "$acceptance_checkout" rev-parse HEAD)" == "$acceptance_commit" ]] || return 2
  [[ -z "$(git -C "$acceptance_checkout" status --porcelain --untracked-files=all)" ]] || return 2
  [[ "$(git -C "$acceptance_checkout" remote get-url origin)" == "https://github.com/$acceptance_repo.git" ]] || return 2
}

acceptance_actions_policy() {
  gh api --hostname github.com "repos/$acceptance_repo/actions/permissions" --jq \
    '[.enabled, (if has("allowed_actions") then .allowed_actions else "absent" end), (if has("sha_pinning_required") then .sha_pinning_required else "absent" end)]|@tsv'
}

acceptance_capture_actions() {
  local policy
  policy="$(acceptance_actions_policy)"
  IFS=$'\t' read -r actions_enabled actions_allowed actions_pinning <<< "$policy"
  case "$actions_enabled:$actions_allowed" in true:all|false:all|false:absent) ;; *)
    printf 'Unreviewed Actions policy; preserve it and inspect before publication.\n' >&2; return 2 ;;
  esac
  case "$actions_pinning" in true|false|absent) ;; *) return 2 ;; esac
  {
    printf '{"enabled":%s' "$actions_enabled"
    if [[ "$actions_allowed" != absent ]]; then printf ',"allowed_actions":"%s"' "$actions_allowed"; fi
    if [[ "$actions_pinning" != absent ]]; then printf ',"sha_pinning_required":%s' "$actions_pinning"; fi
    printf '}\n'
  } > "$acceptance_output/actions-before.json"
}

acceptance_verify_enabled_policy() {
  local policy enabled allowed pinning
  if ! policy="$(acceptance_actions_policy)"; then
    acceptance_restore_actions
    return 2
  fi
  IFS=$'\t' read -r enabled allowed pinning <<< "$policy"
  if [[ "$enabled:$allowed" == true:all && "$pinning" == "$actions_pinning" ]]; then return; fi
  printf 'Enabled Actions policy differs from the reviewed contract; restoring without a push.\n' >&2
  acceptance_restore_actions
  printf 'policy-review-required\n' > "$acceptance_output/status"
  return 2
}

acceptance_restore_actions() {
  # Restore the captured repository policy before re-enabling formerly active workflows.
  gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/permissions" \
    --input "$acceptance_output/actions-before.json"
  while IFS=$'\t' read -r workflow_id workflow_state workflow_path; do
    [[ "$workflow_id" =~ ^[0-9]+$ ]] || return 2
    local current_state current_path
    IFS=$'\t' read -r current_state current_path < <(gh api --hostname github.com \
      "repos/$acceptance_repo/actions/workflows/$workflow_id" --jq '[.state,.path]|@tsv')
    [[ "$current_path" == "$workflow_path" ]] || return 2
    case "$workflow_state:$current_state" in
      active:active|disabled_*:disabled_*) continue ;;
      active:disabled_*|disabled_*:active) ;;
      *) printf 'Unknown workflow state transition: %s -> %s\n' "$current_state" "$workflow_state" >&2; return 2 ;;
    esac
    if [[ "$workflow_state" == active ]]; then
      gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/workflows/$workflow_id/enable"
    else
      gh api --hostname github.com --method PUT "repos/$acceptance_repo/actions/workflows/$workflow_id/disable"
    fi
  done < "$acceptance_output/workflows-before.tsv"
  printf 'restored\n' > "$acceptance_output/actions-state"
}
