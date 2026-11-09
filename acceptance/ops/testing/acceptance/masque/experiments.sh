#!/usr/bin/env bash
# The same input contract and namespace runner as lab-masque-netem.
masque_experiment() {
  local round="$1" policy="$2" run outcome=0
  run="$report/runs/$round-$policy"
  mkdir -m 0700 "$run" "$run/experiment"
  netem_prepare_runtime "$root" "$candidate" "$arch" "$run/experiment" > "$run/runtime-integrity.log"
  cp "$report/network.test" "$root/source.txt" "$run/experiment/"
  cp "$root"/ops/testing/diagnostics/netem/{guest,namespace,context}.sh "$run/experiment/"
  printf 'owned-native-ci\n' > "$run/experiment/execution-context"
  printf '8\n' > "$run/experiment/cycles"
  printf '8\n' > "$run/experiment/workers"
  printf '32\n' > "$run/experiment/packets"
  if [[ "$policy" == default ]]; then printf '\n'; else printf 'reno\n'; fi > "$run/experiment/congestion"
  (cd "$run/experiment"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$run/SHA256SUMS"
  cp "$run/SHA256SUMS" "$run/experiment/"
  (cd "$run/experiment"; bash guest.sh) > "$run/guest.log" 2>&1 || outcome=$?
  printf '%s\n' "$outcome" > "$run/exit-code"
  printf 'MASQUE comparison round=%s policy=%s exit=%s\n' "$round" "$policy" "$outcome"
}

masque_evidence_inventory() {
  (cd "$report"; find . -type f ! -path ./evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > "$report/evidence.sha256"
}
