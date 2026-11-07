#!/usr/bin/env bash
# Shared preparation and provenance for standalone tests delivered to the private VM.
package_begin() {
  arch="$1"
  case "$arch" in amd64|arm64) ;; *) printf 'Unsupported Linux architecture.\n' >&2; exit 2 ;; esac
  [[ ! -e "$2" && ! -L "$2" ]] || { printf 'Package output must be a new directory.\n' >&2; exit 2; }
  [[ -d "$(dirname "$2")" ]] || { printf 'Package parent directory must already exist.\n' >&2; exit 2; }
  output="$(cd "$(dirname "$2")" && pwd -P)/$(basename "$2")"
  case "$output" in "$root/build/"*) ;; *) printf 'Package output must be under build/.\n' >&2; exit 2 ;; esac
  mkdir "$output"
  printf 'preparing\n' > "$output/status"
  trap 'result=$?; if ((result != 0)); then printf "failed\n" > "$output/status"; fi' EXIT
  bundle="$output/payload"
  mkdir -p "$bundle/bin" "$bundle/internal/control/integration" "$bundle/examples/nhop"
}

package_test() {
  GOOS=linux GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go test -c -trimpath -buildvcs=false \
    -ldflags=-buildid= -o "$bundle/integration.test" "$root/internal/control/integration"
  cp "$root/examples/nhop/topology.yaml" "$bundle/examples/nhop/"
}

package_seal() {
  {
    printf 'mode=%s\ntarget=linux/%s\nrace_detector=false\n' "$1" "$arch"
    go version
    git -C "$root" rev-parse HEAD
    git -C "$root" status --porcelain --untracked-files=normal
  } > "$bundle/provenance.txt"
  (cd "$root"; find cmd internal -type f -name '*.go' -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256; shasum -a 256 go.mod go.sum ops/testing/linux/*.sh) > "$bundle/source.sha256"
  (cd "$bundle"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) > "$output/SHA256SUMS"
  cp "$output/SHA256SUMS" "$bundle/SHA256SUMS"
  COPYFILE_DISABLE=1 tar -czf "$output/payload.tar.gz" -C "$output" payload
  shasum -a 256 "$output/payload.tar.gz" > "$output/archive.sha256"
  printf 'ready\n' > "$output/status"
  printf 'Standalone Linux %s: %s\n' "$1" "$output/payload.tar.gz"
}
