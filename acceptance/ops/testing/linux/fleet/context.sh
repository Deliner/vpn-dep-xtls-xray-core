#!/usr/bin/env bash
# Source-only host identity check, run before staging any privileged Fleet files.
fleet_execution_context() {
  [[ "$(uname -s)" == Linux && -d /run/systemd/system ]] || return 2
  case "$mode" in
    private-lima)
      [[ "$(hostname)" == lima-payload && "$PWD" == /home/nhop/runs/run-*/payload ]] || return 2
      ;;
    owned-native-ci)
      [[ "${GITHUB_ACTIONS:-}" == true && "${RUNNER_ENVIRONMENT:-}" == github-hosted ]] || return 2
      [[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core ]] || return 2
      [[ "${GITHUB_REPOSITORY_ID:-}" == 1449369505 && "${GITHUB_REPOSITORY_OWNER_ID:-}" == 26311467 ]] || return 2
      [[ "${GITHUB_EVENT_NAME:-}" == push && "${GITHUB_REF:-}" == refs/heads/nhop-acceptance ]] || return 2
      [[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ && "${GITHUB_RUN_ID:-}" =~ ^[1-9][0-9]*$ ]] || return 2
      case "${NHOP_NODE_CI:-}/$(uname -m)" in linux/amd64/x86_64|linux/arm64/aarch64) ;; *) return 2 ;; esac
      [[ "${GITHUB_WORKSPACE:-}" == /* && "$PWD" == "$GITHUB_WORKSPACE/acceptance/build/fleet-acceptance/package" ]] || return 2
      ;;
    *) return 2 ;;
  esac
}
