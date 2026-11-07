#!/usr/bin/env bash
candidate_configure() {
  candidate_patch
  build_label=nhop-fork-udp-1
  build_package=./main
  build_tags=
  build_cgo=0
  build_flags="-s -w -buildid= -X github.com/xtls/xray-core/core.build=$build_label"
}

candidate_patch() {
  local patch="$recipe/ephemeral-ports.patch"
  local digest=6eda7c04ae5ce2a80473276a0dbe60b7aa34c3c496b3fc203b1e344aa9219a34
  local target="$work/source/transport/internet/system_listener.go" before after state
  [[ -f "$patch" && ! -L "$patch" ]] || { printf 'Candidate patch must be a regular file.\n' >&2; return 2; }
  printf '%s  %s\n' "$digest" "$patch" | shasum -a 256 --check
  before="$(shasum -a 256 "$target" | awk '{print $1}')"
  if git -C "$work/source" apply --reverse --check --whitespace=error-all "$patch" 2>/dev/null; then
    state=already-in-source
  else
    git -C "$work/source" apply --check --whitespace=error-all "$patch"
    git -C "$work/source" apply --whitespace=error-all "$patch"
    state=applied-by-recipe
  fi
  after="$(shasum -a 256 "$target" | awk '{print $1}')"
  [[ "$state" == already-in-source || "$before" != "$after" ]] || { printf 'Candidate patch did not change its target.\n' >&2; return 2; }
  printf 'patch_file=ops/runtime/xray/ephemeral-ports.patch\npatch_sha256=%s\npatch_state=%s\npatch_target_before_sha256=%s\npatch_target_after_sha256=%s\n' "$digest" "$state" "$before" "$after" > "$work/patch.txt"
}

candidate_companions() { :; }
