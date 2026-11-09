#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ "$(id -u)" != 0 ]] || exit 2
sha256sum --check SHA256SUMS
source ./context.sh
netem_execution_context
for namespace in net mnt pid; do
  original="$(readlink "/proc/self/ns/$namespace")"
  [[ -n "$original" ]] || exit 2
  export "NHOP_MASQUE_HOST_$namespace=$original"
done
cycles="$(cat cycles)" workers="$(cat workers)" packets="$(cat packets)"
congestion="$(cat congestion)"
case "$congestion" in ''|bbr|reno) ;; *) exit 2 ;; esac
scenario=network-faults
if [[ -e scenario || -L scenario ]]; then
  [[ -f scenario && ! -L scenario ]] || exit 2
  scenario="$(cat scenario)"
fi
case "$scenario" in network-faults|delay-history) ;; *) exit 2 ;; esac
jitter=0
queue=time
traffic=mixed
if [[ "$scenario" == delay-history ]]; then
  [[ -f delay-jitter-ms && ! -L delay-jitter-ms ]] || exit 2
  jitter="$(cat delay-jitter-ms)"
  case "$jitter" in 0|5) ;; *) exit 2 ;; esac
  [[ -f delay-queue && ! -L delay-queue ]] || exit 2
  queue="$(cat delay-queue)"
  case "$queue" in time|fifo) ;; *) exit 2 ;; esac
  [[ -f delay-traffic && ! -L delay-traffic ]] || exit 2
  traffic="$(cat delay-traffic)"
  case "$traffic" in mixed|tcp|udp) ;; *) exit 2 ;; esac
fi
runtime_kind="$(cat runtime-kind)"
case "$runtime_kind" in diagnostic-probe|runtime-candidate) ;; *) exit 2 ;; esac
[[ "$cycles" =~ ^[1-9][0-9]?$ && "$workers" =~ ^[1-9][0-9]?$ && "$packets" =~ ^[1-9][0-9]{0,2}$ ]] || exit 2
((cycles <= 20 && workers <= 64 && packets <= 256)) || exit 2
before="$(netem_host_network)"
result=0
identity=()
for key in GITHUB_ACTIONS GITHUB_REPOSITORY GITHUB_REPOSITORY_ID GITHUB_REPOSITORY_OWNER_ID RUNNER_ENVIRONMENT \
  GITHUB_EVENT_NAME GITHUB_REF GITHUB_SHA GITHUB_RUN_ID GITHUB_WORKSPACE NHOP_NODE_CI; do
  identity+=("$key=${!key:-}")
done
# Private PID namespace kills every descendant even if the test times out or exits abruptly.
sudo -n env NHOP_MASQUE_HOST_net="$NHOP_MASQUE_HOST_net" NHOP_MASQUE_HOST_mnt="$NHOP_MASQUE_HOST_mnt" \
  NHOP_MASQUE_HOST_pid="$NHOP_MASQUE_HOST_pid" NHOP_MASQUE_CYCLES="$cycles" NHOP_MASQUE_WORKERS="$workers" \
  NHOP_MASQUE_PACKETS="$packets" NHOP_MASQUE_CONGESTION="$congestion" NHOP_MASQUE_RUNTIME_KIND="$runtime_kind" \
  NHOP_MASQUE_SCENARIO="$scenario" NHOP_MASQUE_DELAY_JITTER_MS="$jitter" \
  NHOP_MASQUE_DELAY_QUEUE="$queue" NHOP_MASQUE_DELAY_TRAFFIC="$traffic" \
  "${identity[@]}" unshare --mount --net --pid --fork --mount-proc --kill-child \
  bash namespace.sh || result=$?
[[ "$(netem_host_network)" == "$before" ]] || result=1
if [[ -d reports && ! -L reports ]]; then
  # All private namespace processes are gone; only this fresh result tree is handed back.
  [[ -z "$(sudo -n find reports ! -type f ! -type d -print)" ]] || exit 1
  sudo -n chown -R "$(id -u):$(id -g)" reports
  sha256sum --check SHA256SUMS > reports/input-integrity-after.txt || result=1
  if ((result != 0)); then printf 'failed\n' > reports/status; fi
  (cd reports; find . -type f ! -name evidence.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) > reports/evidence.sha256
  cat reports/experiment.log
fi
exit "$result"
