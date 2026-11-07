#!/usr/bin/env bash
# Exercise controller SSH/SCP only against this checkout's already running VM.
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/linux/environment.sh"
lab_environment
lab_owned
[[ "$(lab_shell hostname)" == lima-payload ]] || { printf 'Start the owned VM with just lab-start.\n' >&2; exit 2; }
config="$LIMA_HOME/$lab_instance/ssh.config"
port="$(ssh -G -F "$config" "lima-$lab_instance" 2>/dev/null | awk '$1 == "port" { print $2 }')"
[[ "$port" =~ ^[0-9]+$ ]] || { printf 'Could not resolve the private VM SSH port.\n' >&2; exit 2; }
stage="$(mktemp -d "$LIMA_HOME/controller-ssh-XXXXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
[[ -f "$LIMA_HOME/_config/user" && ! -L "$LIMA_HOME/_config/user" ]] || exit 2
cp "$LIMA_HOME/_config/user" "$stage/identity"
# Reuse the VM's previously authenticated host key; never scan or accept a new one.
awk -v port="$port" '$1 ~ /(^|,)lima-payload(,|$)/ { print "[127.0.0.1]:" port " " $2 " " $3 }' \
  "$LIMA_HOME/$lab_instance/ssh.known_hosts" > "$stage/known_hosts"
[[ -s "$stage/known_hosts" ]] || { printf 'The owned VM has no previously authenticated host key.\n' >&2; exit 2; }
export NHOP_SSH_LAB_ROOT="$stage" NHOP_SSH_LAB_PORT="$port"
mkdir -p "$lab_data/runs"
report="$(mktemp -d "$lab_data/runs/ssh-XXXXXXXX")"
CGO_ENABLED=1 go test -race -count=1 -v ./internal/platform/sshutil -run '^TestPrivateLabSSH$' | tee "$report/host.log"
GOOS=linux GOARCH="$lab_arch" CGO_ENABLED=0 go test -c -trimpath -buildvcs=false \
  -o "$report/ssh.test" ./internal/platform/sshutil
run_name="$(basename "$report")"
lab_shell mkdir -p "runs/$run_name"
"$lab_lima" copy "$report/ssh.test" "$lab_instance:/home/nhop/runs/$run_name/ssh.test"
lab_shell "runs/$run_name/ssh.test" -test.run '^TestControllerSSHDoesNotInheritForeignConfiguration$' \
  -test.v -test.timeout=30s | tee "$report/linux-parser.log"
{
  ssh -V
  lab_shell ssh -V
  shasum -a 256 "$report/ssh.test" "$lab_root/internal/platform/sshutil/"*.go "$lab_root/ops/testing/ssh.sh"
} > "$report/provenance.txt" 2>&1
printf 'SSH evidence: %s\n' "$report"
