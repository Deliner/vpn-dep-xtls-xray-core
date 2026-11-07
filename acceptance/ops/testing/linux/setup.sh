#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/environment.sh"
lab_environment
mkdir -p "$lab_data/downloads"
[[ ! -L "$HOME/.ssh-custom" && ! -L "$LIMA_HOME" ]] || { printf 'SSH lab directories must not be symlinks.\n' >&2; exit 2; }
if [[ -e "$LIMA_HOME" ]]; then
  [[ -f "$LIMA_HOME/.nhop-owner" && "$(cat "$LIMA_HOME/.nhop-owner")" == "$lab_root" ]] || { printf 'Refusing an unowned SSH directory.\n' >&2; exit 2; }
else
  mkdir -p "$HOME/.ssh-custom"
  mkdir -m 0700 "$LIMA_HOME"
  printf '%s\n' "$lab_root" > "$LIMA_HOME/.nhop-owner"
fi

fetch_lab_input() {
  local component="$1" platform="$2" output="$3" row url digest stage
  row="$(awk -F '\t' -v c="$component" -v p="$platform" '$1 == c && $2 == p { print $3 " " $4 }' "$lab_root/ops/testing/linux/pins.tsv")"
  read -r url digest <<< "$row"
  [[ "$url" == https://* && "$digest" =~ ^[a-f0-9]{64}$ ]] || return 2
  if [[ -f "$output" ]]; then
    printf '%s  %s\n' "$digest" "$output" | shasum -a 256 --check
    return
  fi
  stage="$output.partial"
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 900 \
    --proto '=https' --proto-redir '=https' --output "$stage" "$url"
  printf '%s  %s\n' "$digest" "$stage" | shasum -a 256 --check
  mv "$stage" "$output"
}

fetch_lab_input lima "darwin/$lab_arch" "$lab_data/downloads/lima.tar.gz"
if [[ ! -e "$lab_data/tools" ]]; then
  stage="$(mktemp -d "$lab_data/.tools-XXXXXXXX")"
  trap 'rm -rf "$stage"' EXIT
  tar -xzf "$lab_data/downloads/lima.tar.gz" -C "$stage"
  [[ "$("$stage/bin/limactl" --version)" == 'limactl version 2.3.0' ]] || exit 2
  mv "$stage" "$lab_data/tools"
  trap - EXIT
fi
[[ "$("$lab_lima" --version)" == 'limactl version 2.3.0' ]] || exit 2
fetch_lab_input ubuntu "linux/$lab_arch" "$lab_data/downloads/ubuntu.img"
image_digest="$(awk -F '\t' -v p="linux/$lab_arch" '$1 == "ubuntu" && $2 == p { print $4 }' "$lab_root/ops/testing/linux/pins.tsv")"
# JSON strings are valid YAML scalars; quote paths without shell evaluation.
image_path="$(printf '%s' "$lab_data/downloads/ubuntu.img" | sed 's/\\/\\\\/g; s/"/\\"/g')"
cat > "$lab_data/config.yaml" <<EOF
vmType: vz
arch: $lab_guest_arch
cpus: 4
memory: 4GiB
disk: 8GiB
images:
- location: "$image_path"
  arch: $lab_guest_arch
  digest: "sha256:$image_digest"
mounts: []
plain: true
containerd: {system: false, user: false}
ssh: {loadDotSSHPubKeys: false, forwardAgent: false, forwardX11: false, overVsock: false}
networkOpts: {hostLoopback: false}
user: {name: nhop, uid: 1000, home: /home/nhop, shell: /bin/bash}
vmOpts:
  vz:
    diskImageFormat: raw
    rosetta: {enabled: false, binfmt: false}
EOF
if [[ ! -e "$LIMA_HOME/$lab_instance" ]]; then
  "$lab_lima" create --tty=false --name="$lab_instance" "$lab_data/config.yaml"
else
  cmp "$lab_data/config.yaml" "$LIMA_HOME/$lab_instance/lima.yaml" || { printf 'Existing VM configuration differs; inspect before reusing it.\n' >&2; exit 2; }
fi
disk="$LIMA_HOME/$lab_instance/disk"
if [[ ! -L "$disk" ]]; then
  [[ ! -e "$lab_data/disk" ]] || { printf 'Refusing to replace an existing lab disk.\n' >&2; exit 2; }
  [[ "$("$lab_lima" list --format '{{.Status}}' "$lab_instance")" == Stopped ]] || { printf 'Stop the VM before relocating its initial disk.\n' >&2; exit 2; }
  mv "$disk" "$lab_data/disk"
  ln -s "$lab_data/disk" "$disk"
fi
printf 'Private lab prepared. Start with: just lab-start\n'
