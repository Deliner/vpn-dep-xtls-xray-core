#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/identity.sh"
[[ $# == 2 && "$2" =~ ^[0-9]+$ ]] || exit 2
acceptance_identity
acceptance_directory "$1"
run_id="$2"
gh run view "$run_id" --repo "$acceptance_repo" --json databaseId,headSha,headBranch,status,conclusion,url,event > "$acceptance_output/run.json"
[[ "$(gh run view "$run_id" --repo "$acceptance_repo" --json headSha,headBranch,status,event --jq '[.headSha,.headBranch,.status,.event]|@tsv')" == "$acceptance_commit"$'\tnhop-acceptance\tcompleted\tpush' ]] || { printf 'Run identity differs or the run is still live.\n' >&2; exit 2; }
collection="$(mktemp -d "$acceptance_output/collection-XXXXXXXX")"
gh run download "$run_id" --repo "$acceptance_repo" --dir "$collection"
gh run view "$run_id" --repo "$acceptance_repo" --log > "$collection/workflow.log"
acceptance_restore_actions
[[ "$(gh run view "$run_id" --repo "$acceptance_repo" --json conclusion --jq .conclusion)" == success ]] || { printf 'Workflow failed; preserved results: %s\n' "$collection" >&2; exit 1; }
for arch in amd64 arm64; do
  report="$collection/acceptance-linux-$arch"
  [[ "$(cat "$report/status")" == passed ]] || exit 1
  (cd "$report"; shasum -a 256 --check evidence.sha256) > "$report/integrity-verification.log"
  started="$(cat "$report/started-unix")"
  just payload-report "$report/payload" '1,3' "$started" "linux/$arch" '127.0.0.1'
  for scenario in ipv6-single ipv6-chain mixed-6-4-6 mixed-4-6-4; do
    hops=3
    addresses=::1
    case "$scenario" in ipv6-single) hops=1 ;; mixed-6-4-6) addresses=::1,127.0.0.1,::1 ;; mixed-4-6-4) addresses=127.0.0.1,::1,127.0.0.1 ;; esac
    just payload-report "$report/payload/$scenario" "$hops" "$started" "linux/$arch" "$addresses"
  done
done
printf 'verified\n' > "$acceptance_output/status"
printf 'Independent acceptance verification passed: %s\n' "$collection"
