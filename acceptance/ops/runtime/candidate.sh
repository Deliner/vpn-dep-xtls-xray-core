#!/usr/bin/env bash
set -euo pipefail
umask 077

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "$root/ops/runtime/integrity.sh"
action="${1:-}"
engine="${2:-}"
case "$engine" in xray|sing-box) ;; *) printf 'Supported candidate engines: xray, sing-box\n' >&2; exit 2 ;; esac
recipe="$root/ops/runtime/$engine"
source "$recipe/build.sh"
[[ "$(go env GOVERSION)" == go1.27.3 ]] || { printf 'Candidate requires Go 1.27.3\n' >&2; exit 2; }
export GOWORK=off GOTOOLCHAIN=local GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GOPRIVATE= GONOPROXY= GONOSUMDB=
IFS=$'\t' read -r version url source_sha source_size < <(awk '!/^#/ && NF { print; exit }' "$recipe/source.tsv")
build_label=nhop-security-1
[[ ! -L "$root/build" && ! -L "$root/build/runtime-candidates" ]] || { printf 'Output parent may not be a symlink\n' >&2; exit 2; }

prepare_source() {
  mkdir -p "$work/source"
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 \
    --proto '=https' --proto-redir '=https' --output "$work/source.tar.gz" "$url"
  [[ "$(wc -c < "$work/source.tar.gz" | tr -d ' ')" == "$source_size" ]]
  printf '%s  %s\n' "$source_sha" "$work/source.tar.gz" | shasum -a 256 --check
  # This exact, reviewed archive contains only regular files and directories.
  # Verify its pin before letting tar extract anything.
  tar -xzf "$work/source.tar.gz" --strip-components=1 -C "$work/source"
}

refresh_lock() {
  local module version
  local modules=()
  while IFS=$'\t' read -r module version; do
    [[ -n "$module" && "$module" != \#* ]] || continue
    modules+=("$module@$version")
  done < "$recipe/dependencies.tsv"
  GOFLAGS=-mod=mod go -C "$work/source" get "${modules[@]}"
  GOFLAGS=-mod=mod go -C "$work/source" mod tidy
  cp "$work/source/go.mod" "$recipe/go.mod"
  cp "$work/source/go.sum" "$recipe/go.sum"
  printf 'Updated %s locks; review both files before building.\n' "$recipe"
}

build_candidate() {
  cp "$recipe/go.mod" "$recipe/go.sum" "$work/source/"
  mkdir "$work/bin"
  candidate_configure
  GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED="$build_cgo" GOFLAGS=-mod=readonly \
    go -C "$work/source" build -trimpath -buildvcs=false -tags "$build_tags" -ldflags "$build_flags" -o "$work/bin/$engine" "$build_package"
  candidate_companions
  go -C "$work/source" mod verify
  candidate_inventory "$work" > "$work/SHA256SUMS"
  {
    printf 'engine=%s\nbase_version=%s\nbuild_label=%s\ntarget=%s/%s\n' "$engine" "$version" "$build_label" "$target_os" "$target_arch"
    printf 'source_url=%s\nsource_archive_sha256=%s\n' "$url" "$source_sha"
    printf 'build_tags=%s\nbuild_cgo=%s\n' "$build_tags" "$build_cgo"
    go version
    go version -m "$work/bin/$engine"
    (cd "$work" && shasum -a 256 source/go.mod source/go.sum bin/*)
    (cd "$root" && shasum -a 256 ops/runtime/candidate.sh ops/runtime/integrity.sh "ops/runtime/$engine/build.sh" "ops/runtime/$engine/"*.tsv)
    if [[ -f "$work/companion.txt" ]]; then cat "$work/companion.txt"; fi
    if [[ -f "$work/patch.txt" ]]; then cat "$work/patch.txt"; fi
  } > "$work/provenance.txt"
  printf 'built\n' > "$work/status"
  printf 'Candidate: %s/bin/%s\nProvenance: %s/provenance.txt\n' "$work" "$engine" "$work"
}

case "$action" in
  lock)
    [[ $# == 2 ]] || exit 2
    mkdir -p "$root/build/runtime-candidates"
    work="$(mktemp -d "$root/build/runtime-candidates/.lock-XXXXXXXX")"
    trap 'rm -rf "$work"' EXIT
    prepare_source
    refresh_lock
    ;;
  build)
    [[ $# == 5 ]] || exit 2
    target_os="$3"; target_arch="$4"
    case "$target_os/$target_arch" in linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;; *) exit 2 ;; esac
    # A fresh directory is a run boundary: no previous candidate is overwritten.
    case "$5" in build/runtime-candidates/*) ;; *) printf 'Output must be under build/runtime-candidates/\n' >&2; exit 2 ;; esac
    name="${5#build/runtime-candidates/}"
    case "$name" in ''|.|..|*[!a-zA-Z0-9._-]*) printf 'Output must have a simple directory name\n' >&2; exit 2 ;; esac
    mkdir -p "$root/build/runtime-candidates"
    mkdir "$root/$5"
    work="$(cd "$root/$5" && pwd -P)"
    case "$work/" in "$root/build/"*) ;; *) printf 'Output escapes build/\n' >&2; exit 2 ;; esac
    printf 'building\n' > "$work/status"
    trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$work/status"; fi' EXIT
    prepare_source
    build_candidate
    ;;
  *) printf 'usage: candidate.sh lock ENGINE | build ENGINE OS ARCH NEW_BUILD_DIRECTORY\n' >&2; exit 2 ;;
esac
