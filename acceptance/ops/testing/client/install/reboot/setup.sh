#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
reboot_lab_environment
[[ -x "$lab_lima" && "$("$lab_lima" --version)" == 'limactl version 2.3.0' ]] || exit 2
[[ ! -L "$HOME/.ssh-custom" && ! -L "$LIMA_HOME" && ! -L "$reboot_data" ]] || exit 2
if [[ -e "$LIMA_HOME" ]]; then
  lab_owned
else
  mkdir -m 0700 "$LIMA_HOME"
  printf '%s\n' "$lab_root" > "$LIMA_HOME/.nhop-owner"
fi
mkdir -p "$reboot_data/disks" "$reboot_data/runs"
[[ ! -L "$reboot_data/config.yaml" ]] || exit 2
network="$("$lab_lima" network list --json | awk -F '"' '$4 == "client-boot"')"
if [[ -z "$network" ]]; then
  "$lab_lima" network create client-boot --mode=user-v2 --gateway=192.168.125.1/24
fi
network="$("$lab_lima" network list --json | awk -F '"' '$4 == "client-boot"')"
[[ "$network" == '{"name":"client-boot","mode":"user-v2","gateway":"192.168.125.1","netmask":"255.255.255.0"}' ]] || {
  printf 'Refusing a different client reboot network configuration.\n' >&2; exit 2;
}
digest="$(awk -F '\t' -v p="linux/$lab_arch" '$1 == "ubuntu" && $2 == p { print $4 }' "$lab_root/ops/testing/linux/pins.tsv")"
[[ "$digest" =~ ^[0-9a-f]{64}$ ]] || exit 2
printf '%s  %s\n' "$digest" "$lab_data/downloads/ubuntu.img" | shasum -a 256 --check
image="$(printf '%s' "$lab_data/downloads/ubuntu.img" | sed 's/\\/\\\\/g; s/"/\\"/g')"
cat > "$reboot_data/config.yaml" <<EOF
vmType: vz
arch: $lab_guest_arch
cpus: 2
memory: 2GiB
disk: 8GiB
images:
- location: "$image"
  arch: $lab_guest_arch
  digest: "sha256:$digest"
mounts: []
plain: true
containerd: {system: false, user: false}
ssh: {loadDotSSHPubKeys: false, forwardAgent: false, forwardX11: false, overVsock: false}
networks:
- lima: client-boot
user: {name: nhop, uid: 1000, home: /home/nhop, shell: /bin/bash}
vmOpts:
  vz:
    diskImageFormat: raw
    rosetta: {enabled: false, binfmt: false}
EOF
for node in "${reboot_nodes[@]}"; do
  if [[ ! -e "$LIMA_HOME/$node" ]]; then
    "$lab_lima" create --tty=false --name="$node" "$reboot_data/config.yaml"
  else
    cmp "$reboot_data/config.yaml" "$LIMA_HOME/$node/lima.yaml"
  fi
  disk="$LIMA_HOME/$node/disk"
  if [[ ! -L "$disk" ]]; then
    [[ ! -e "$reboot_data/disks/$node" && "$("$lab_lima" list --format '{{.Status}}' "$node")" == Stopped ]] || exit 2
    mv "$disk" "$reboot_data/disks/$node"
    ln -s "$reboot_data/disks/$node" "$disk"
  fi
done
reboot_lab_owned
printf 'Two private client reboot VM disks prepared; start with just client-reboot-lab-start.\n'
