#!/usr/bin/env bash
# Source-only host bounds, shared by the VM and native CI namespace launcher.
netem_native_identity() {
  [[ "${GITHUB_ACTIONS:-}" == true && "${RUNNER_ENVIRONMENT:-}" == github-hosted ]] || return 2
  [[ "${GITHUB_REPOSITORY:-}" == Deliner/vpn-dep-xtls-xray-core ]] || return 2
  [[ "${GITHUB_REPOSITORY_ID:-}" == 1449369505 && "${GITHUB_REPOSITORY_OWNER_ID:-}" == 26311467 ]] || return 2
  [[ "${GITHUB_EVENT_NAME:-}" == push && "${GITHUB_REF:-}" == refs/heads/nhop-acceptance ]] || return 2
  [[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ && "${GITHUB_RUN_ID:-}" =~ ^[1-9][0-9]*$ ]] || return 2
  case "${NHOP_NODE_CI:-}/$(uname -m)" in linux/amd64/x86_64|linux/arm64/aarch64) ;; *) return 2 ;; esac
}

netem_execution_context() {
  [[ "$(uname -s)" == Linux ]] || return 2
  case "$(cat execution-context)" in
    private-lima)
      [[ "$(hostname)" == lima-payload && "$PWD" == /home/nhop/runs/masque-netem-*/experiment ]] || return 2
      export NHOP_SYSTEMD_ACCEPTANCE=private-lima
      ;;
    owned-native-ci)
      netem_native_identity || return
      [[ "${GITHUB_WORKSPACE:-}" == /* && "$PWD" == "$GITHUB_WORKSPACE/acceptance/build/masque-acceptance/runs/"*/experiment ]] || return 2
      export NHOP_SYSTEMD_ACCEPTANCE=owned-native-ci
      ;;
    *) return 2 ;;
  esac
}

netem_host_network() {
  ip -4 -j route show table all || return
  ip -4 -j rule || return
  ip -6 -j route show table all || return
  ip -6 -j rule || return
  sha256sum /etc/resolv.conf
}
