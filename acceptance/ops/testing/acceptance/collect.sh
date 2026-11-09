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
acceptance_restore_actions
collection="$(mktemp -d "$acceptance_output/collection-XXXXXXXX")"
gh run download "$run_id" --repo "$acceptance_repo" --dir "$collection"
gh run view "$run_id" --repo "$acceptance_repo" --log > "$collection/workflow.log"
if [[ "$(gh run view "$run_id" --repo "$acceptance_repo" --json conclusion --jq .conclusion)" != success ]]; then
  printf 'workflow-failed\n' > "$acceptance_output/status"
  printf 'Workflow failed; preserved results: %s\n' "$collection" >&2
  exit 1
fi
suite=protocols
if [[ -f "$acceptance_output/suite" ]]; then suite="$(cat "$acceptance_output/suite")"; fi
if [[ "$suite" == cronet ]]; then
  for arch in amd64 arm64; do
    report="$collection/cronet-linux-$arch"
    [[ "$(cat "$report/status")" == native-built && -s "$report/bin/libcronet.so" && -s "$report/inspection/elf.txt" ]] || exit 1
    (cd "$report"; shasum -a 256 --check evidence.sha256) > "$report/integrity-verification.log"
    (cd "$report"; shasum -a 256 --check SHA256SUMS) >> "$report/integrity-verification.log"
    cmp "$report/source.tsv" <(sed '/^#/d' "$acceptance_checkout/acceptance/ops/runtime/cronet/source.tsv")
    if [[ -f "$acceptance_checkout/acceptance/ops/testing/acceptance/cronet-verify.sh" || -f "$acceptance_checkout/acceptance/ops/testing/acceptance/cronet/verify.sh" ]]; then
      verification="$collection/cronet-native-$arch"
      [[ "$(cat "$verification/status")" == passed && -s "$verification/range.log" ]] || exit 1
      (cd "$verification"; shasum -a 256 --check evidence.sha256) > "$verification/integrity-verification.log"
      if [[ -f "$acceptance_checkout/acceptance/ops/testing/acceptance/cronet-payload.sh" || -f "$acceptance_checkout/acceptance/ops/testing/acceptance/cronet/payload.sh" ]]; then
        [[ "$(cat "$verification/naive/status")" == passed ]] || exit 1
        (cd "$verification/naive"; shasum -a 256 --check evidence.sha256; shasum -a 256 --check runtime.sha256) >> "$verification/integrity-verification.log"
        cmp "$verification/naive/bin/libcronet.so" "$report/bin/libcronet.so"
      fi
    fi
  done
  printf 'build-evidence-verified\n' > "$acceptance_output/status"
  printf 'Native build and configured regression/payload evidence verified; C/C++ audit remains open: %s\n' "$collection"
  exit 0
fi
if [[ "$suite" == client ]]; then
  bash "$acceptance_root/ops/testing/acceptance/client/collect.sh" "$collection" "$acceptance_commit" "$acceptance_checkout/acceptance"
  printf 'verified\n' > "$acceptance_output/status"
  printf 'Native Linux client acceptance verified: %s\n' "$collection"
  exit 0
fi
if [[ "$suite" == installer ]]; then
  bash "$acceptance_root/ops/testing/acceptance/installer/collect.sh" "$collection" "$acceptance_commit" "$acceptance_checkout/acceptance"
  printf 'verified\n' > "$acceptance_output/status"
  printf 'Native Linux installer acceptance verified: %s\n' "$collection"
  exit 0
fi
if [[ "$suite" == masque ]]; then
  bash "$acceptance_root/ops/testing/acceptance/masque/collect.sh" "$collection" "$acceptance_commit" "$acceptance_checkout/acceptance"
  printf 'verified\n' > "$acceptance_output/status"
  printf 'Native MASQUE congestion evidence verified: %s\n' "$collection"
  exit 0
fi
[[ "$suite" == protocols ]] || exit 2
for arch in amd64 arm64; do
  report="$collection/acceptance-linux-$arch"
  [[ "$(cat "$report/status")" == passed ]] || exit 1
  if grep -Fq 'xray-masque-regressions.log' "$acceptance_checkout/acceptance/ops/testing/acceptance/run.sh"; then
    [[ -s "$report/xray-regressions-integrity.log" ]] || exit 1
    for regression in TestNHOPHTTP3CapsulesAndDatagrams TestNHOPHTTP3CapsuleValidation \
      TestNHOPHTTP3TruncatedCapsuleClosesRequest TestNHOPHTTP3CapsuleShortBuffer \
      TestNHOPDatagramReceiverBackpressure TestNHOPDatagramReceiverStopsWhileBlocked \
      TestNHOPDatagramReceiverConcurrentStop TestNHOPDatagramReceiverStopBeforeFirstRead; do
      grep -Fq -- "--- PASS: $regression (" "$report/xray-masque-regressions.log" || exit 1
    done
  fi
  (cd "$report"; shasum -a 256 --check evidence.sha256) > "$report/integrity-verification.log"
  if grep -Fq 'xray-congestion-regressions.log' "$acceptance_checkout/acceptance/ops/testing/acceptance/run.sh"; then
    grep -Fq -- '--- PASS: TestNHOPBBRPathMTUDecrease (' "$report/xray-congestion-regressions.log" || exit 1
    grep -Fq -- '--- PASS: TestNHOPBBRRejectsNonpositiveDatagramSize (' "$report/xray-congestion-regressions.log" || exit 1
    for profile in standard conservative aggressive; do
      for state in minimum initial learned maximum; do
        grep -Fq -- "--- PASS: TestNHOPBBRPathMTUDecrease/$profile/$state (" "$report/xray-congestion-regressions.log" || exit 1
      done
    done
  fi
  if grep -Fq 'xray-tcp-regressions.log' "$acceptance_checkout/acceptance/ops/testing/acceptance/run.sh"; then
    for regression in TestNHOPTCPReorderingLossThreshold TestNHOPTCPNewSmallSACKsTriggerRecovery \
      TestNHOPTCPSelectiveACKAfterCumulativeProgress TestNHOPTCPNonSACKDuplicateThreshold \
      TestNHOPTCPReorderingDoesNotRetransmitEarly TestNHOPRemoteTCPReordering \
      TestTCPRetransmit TestTCPLossAgain; do
      grep -Fq -- "--- PASS: $regression (" "$report/xray-tcp-regressions.log" || exit 1
    done
    for base in 000003e8 ffffff80; do
      for scenario in no-sack one-segment two-segments two-islands overlap adjacent \
        past-flight before-ack more-than-two three-islands; do
        grep -Fq -- "--- PASS: TestNHOPTCPReorderingLossThreshold/$base/$scenario (" "$report/xray-tcp-regressions.log" || exit 1
      done
    done
    for address in '10.0.0.1:1234' '[fd00::1]:1234'; do
      grep -Fq -- "--- PASS: TestNHOPTCPReorderingDoesNotRetransmitEarly/$address (" "$report/xray-tcp-regressions.log" || exit 1
    done
  fi
  started="$(cat "$report/started-unix")"
  if grep -Fq 'quic-datagram-regressions.log' "$acceptance_checkout/acceptance/ops/testing/acceptance/run.sh"; then
    grep -Fq 'all modules verified' "$report/quic-module-integrity.log" || exit 1
    for regression in TestNHOPDatagramBudgetConfigCopies TestNHOPDatagramBudgetConfigRejectsInvalidBounds \
      TestNHOPDatagramBudgetDefaultsRemainLocal TestNHOPDatagramReceiveBudgetBounds \
      TestNHOPDatagramReceiveBudgetRejectsWithoutAllocation TestNHOPDatagramReceiveRingWrap \
      TestNHOPDatagramReceiveBudgetConcurrentUse TestNHOPConnectionDatagramBurst; do
      grep -Fq -- "--- PASS: $regression (" "$report/quic-datagram-regressions.log" || exit 1
    done
  fi
  if grep -Fq 'quic-mtu-regressions.log' "$acceptance_checkout/acceptance/ops/testing/acceptance/run.sh"; then
    for regression in TestNHOPMTUReductionBeforeHandshakeKeepsConfigPrivate TestNHOPMTUReductionUpdatesAllSendLimits \
      TestNHOPMTUEpochDoesNotWrapAfterRepeatedReduction TestNHOPMTUReductionIgnoresOldLargerPackets \
      TestNHOPSendQueueReportsMTUWithoutBlocking TestNHOPSendQueueMinimumMTUFailureIsFatal \
      TestNHOPSendQueueShortGSOSegmentFeedback; do
      grep -Fq -- "--- PASS: $regression (" "$report/quic-datagram-regressions.log" || exit 1
    done
    for regression in TestNHOPMTULossEvidenceRequiresRepeatedUnsupportedSize TestNHOPMTUBlackholePTOAndEpoch \
      TestNHOPMTUEvidenceExcludesACKOnlyAndProbePackets TestNHOPMTUMigrationStartsNewSizeEpoch TestNHOPPathMTUDecrease; do
      grep -Fq -- "--- PASS: $regression (" "$report/quic-mtu-regressions.log" || exit 1
    done
    for network in udp4 udp6; do
      for mode in kernel blackhole; do
        for side in client server; do
          grep -Fq -- "--- PASS: TestNHOPPathMTUDecrease/$network/$mode/$side (" "$report/quic-mtu-regressions.log" || exit 1
        done
      done
    done
  fi
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
