#!/usr/bin/env bash
# Read-only contract for native candidate TUIC shutdown evidence.
tuic_verify_candidate() {
  local report="$1" candidate_inventory="$2" arch="$3" early cancel
  [[ "$(cat "$report/status")" == passed ]] || return 1
  grep -Fxq "execution_host=linux/$arch" "$report/provenance.txt" || return 1
  grep -Fxq 'tuic_patch_state=applied' "$report/provenance.txt" || return 1
  cmp "$candidate_inventory" "$report/candidate-before.sha256" || return 1
  cmp "$candidate_inventory" "$report/candidate-after.sha256" || return 1
  (cd "$report"; shasum -a 256 --check evidence.sha256) || return 1
  grep -Fxq 'all modules verified' "$report/module-verify.log" || return 1
  if grep -Eq -- '--- FAIL:|^FAIL|DATA RACE|panic:|build failed' "$report/patched.log"; then return 1; fi
  [[ "$(grep -Fc -- '--- PASS: TestServerShutdown (' "$report/patched.log")" == 20 ]] || return 1
  for early in false true; do
    for cancel in false true; do
      [[ "$(grep -Fc -- "--- PASS: TestServerShutdown/early=$early/cancel=$cancel (" "$report/patched.log")" == 20 ]] || return 1
    done
  done
}
