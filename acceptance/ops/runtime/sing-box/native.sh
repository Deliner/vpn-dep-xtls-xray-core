#!/usr/bin/env bash
# Linux uses the published native build accepted against the unchanged wrapper.
# Module-library provenance remains recorded separately from the loaded library.
candidate_cronet_release() {
  [[ "$target_os" == linux ]] || return 2
  local archive_sha archive_size library_size url
  awk -F '\t' -v platform="linux/$target_arch" '$1 == platform { count++ } END { exit count != 1 }' "$recipe/native.tsv"
  IFS=$'\t' read -r archive_sha archive_size cronet_digest library_size < <(
    awk -F '\t' -v platform="linux/$target_arch" '$1 == platform { print $2 "\t" $3 "\t" $4 "\t" $5 }' "$recipe/native.tsv"
  )
  [[ "$archive_sha" =~ ^[0-9a-f]{64}$ && "$cronet_digest" =~ ^[0-9a-f]{64}$ ]] || return 2
  [[ "$archive_size" =~ ^[1-9][0-9]*$ && "$library_size" =~ ^[1-9][0-9]*$ ]] || return 2
  url="https://github.com/Deliner/vpn-dep-sagernet-cronet-go/releases/download/nhop-cronet-zlib-1/cronet-linux-$target_arch.zip"
  mkdir "$work/cronet"
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 \
    --proto '=https' --proto-redir '=https' --output "$work/cronet/native.zip" "$url"
  [[ "$(wc -c < "$work/cronet/native.zip" | tr -d ' ')" == "$archive_size" ]] || return 2
  printf '%s  %s\n' "$archive_sha" "$work/cronet/native.zip" | shasum -a 256 --check
  cronet_library="$work/cronet/libcronet.so"
  go -C "$root" run ./cmd/runtimepack cronet-library "$root/ops/release/runtime/cronet.json" "$target_arch" "$work/cronet/native.zip" "$cronet_library"
  [[ "$(wc -c < "$cronet_library" | tr -d ' ')" == "$library_size" ]] || return 2
  printf '%s  %s\n' "$cronet_digest" "$cronet_library" | shasum -a 256 --check
  {
    printf 'loaded_companion_url=%s\nloaded_companion_archive_sha256=%s\n' "$url" "$archive_sha"
    printf 'loaded_companion_sha256=%s\nloaded_companion_size=%s\n' "$cronet_digest" "$library_size"
    printf 'loaded_companion_native_run=42637392545\n'
    (cd "$root" && shasum -a 256 ops/runtime/sing-box/native.sh ops/runtime/sing-box/native.tsv ops/release/runtime/cronet.json)
  } >> "$work/companion.txt"
}
