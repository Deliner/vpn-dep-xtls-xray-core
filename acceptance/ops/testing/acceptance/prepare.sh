#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/identity.sh"
source "$(dirname "${BASH_SOURCE[0]}")/sources.sh"
[[ $# -ge 1 && $# -le 2 && "$1" =~ ^build/acceptance/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || exit 2
suite="${2:-protocols}"
case "$suite" in protocols) workflow=workflow.yml ;; cronet) workflow=cronet.yml ;; client) workflow=client/workflow.yml ;; *) exit 2 ;; esac
acceptance_identity
[[ -z "$(git -C "$acceptance_root" status --porcelain --untracked-files=all)" ]] || { printf 'Commit and check the project before preparing CI.\n' >&2; exit 2; }
for directory in "$acceptance_root/build" "$acceptance_root/build/acceptance"; do
  [[ ! -L "$directory" ]] || exit 2
  mkdir -p "$directory"
done
acceptance_output="$acceptance_root/$1"
[[ ! -e "$acceptance_output" && ! -L "$acceptance_output" ]] || exit 2
mkdir "$acceptance_output"
printf '%s\n' "$acceptance_repo" > "$acceptance_output/repository-name"
printf '%s\n' "$suite" > "$acceptance_output/suite"
git -C "$acceptance_root" rev-parse HEAD > "$acceptance_output/project.commit"
acceptance_checkout="$acceptance_output/repository"
git init --initial-branch=nhop-acceptance "$acceptance_checkout"
git -C "$acceptance_checkout" remote add origin "https://github.com/$acceptance_repo.git"
base=63bd9c0dce4674ca45522aa14a172be240193124
remote_status=0
git -C "$acceptance_checkout" ls-remote --exit-code origin refs/heads/nhop-acceptance > "$acceptance_output/remote-before.tsv" || remote_status=$?
if ((remote_status == 0)); then
  base="$(cut -f1 "$acceptance_output/remote-before.tsv")"
elif ((remote_status != 2)); then
  exit "$remote_status"
fi
[[ "$base" =~ ^[0-9a-f]{40}$ ]] || exit 2
git -C "$acceptance_checkout" -c protocol.file.allow=never fetch --depth=1 origin "$base"
git -C "$acceptance_checkout" checkout -B nhop-acceptance FETCH_HEAD
git -C "$acceptance_checkout" rm -r --ignore-unmatch .github/workflows acceptance
acceptance_copy_sources "$acceptance_root" "$acceptance_checkout" "$acceptance_output"
mkdir -p "$acceptance_checkout/.github/workflows"
cp "$acceptance_root/ops/testing/acceptance/$workflow" "$acceptance_checkout/.github/workflows/nhop-acceptance.yml"
git -C "$acceptance_checkout" add .github/workflows/nhop-acceptance.yml
git -C "$acceptance_checkout" commit -m "Run native N-hop acceptance for $(cat "$acceptance_output/project.commit")"
git -C "$acceptance_checkout" rev-parse HEAD > "$acceptance_output/prepared.commit"
printf 'prepared\n' > "$acceptance_output/status"
printf 'Reviewable CI checkout: %s\n' "$acceptance_checkout"
