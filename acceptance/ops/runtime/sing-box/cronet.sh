#!/usr/bin/env bash
# Keep the Go wrapper, generated bindings and native library in one reviewed generation.
candidate_cronet_prepare() {
  local module version=v0.0.0-20261103043844-095cb9e06046
  for module in github.com/sagernet/cronet-go github.com/sagernet/cronet-go/all; do
    [[ "$(go -C "$work/source" list -m -f '{{.Version}}' "$module")" == "$version" ]] || {
      printf 'Cronet wrapper generation changed; review native libraries.\n' >&2; return 2;
    }
  done
  local native_version member size chromium directory sum
  awk -F '\t' -v platform="$target_os/$target_arch" '$1 == platform { count++ } END { exit count != 1 }' "$recipe/companions.tsv"
  IFS=$'\t' read -r native_version member cronet_digest size chromium < <(
    awk -F '\t' -v platform="$target_os/$target_arch" '$1 == platform { print $2 "\t" $3 "\t" $4 "\t" $5 "\t" $6 }' "$recipe/companions.tsv"
  )
  [[ "$native_version" == v0.0.0-20261103043047-3aac4ed44c5c ]] || return 2
  case "$target_os/$member" in linux/libcronet.so|darwin/libcronet.a) ;; *) return 2 ;; esac
  module="github.com/sagernet/cronet-go/lib/${target_os}_${target_arch}"
  [[ "$(go -C "$work/source" list -m -f '{{.Version}}' "$module")" == "$native_version" ]] || return 2
  go -C "$work/source" mod download "$module"
  go -C "$work/source" mod verify
  directory="$(go -C "$work/source" list -m -f '{{.Dir}}' "$module")"
  [[ "$directory" == "$GOMODCACHE/$module@$native_version" && -d "$directory" && ! -L "$directory" ]] || return 2
  [[ -z "$(find "$directory" ! -type f ! -type d -print -quit)" ]] || return 2
  cronet_library="$directory/$member"
  [[ -f "$cronet_library" && ! -L "$cronet_library" ]] || return 2
  [[ "$(wc -c < "$cronet_library" | tr -d ' ')" == "$size" ]] || return 2
  printf '%s  %s\n' "$cronet_digest" "$cronet_library" | shasum -a 256 --check
  [[ "$(sed -n 's/^const Version = "\([^"]*\)"$/\1/p' "$directory/libcronet_purego.go")" == "$chromium" ]] || return 2
  sum="$(go -C "$work/source" list -m -f '{{.Sum}}' "$module")"
  [[ "$sum" == h1:* ]] || return 2
  {
    printf 'cronet_wrapper_version=%s\ncompanion_module=%s@%s\n' "$version" "$module" "$native_version"
    printf 'companion_module_sum=%s\ncompanion_member=%s\n' "$sum" "$member"
    printf 'companion_sha256=%s\ncompanion_size=%s\ncompanion_chromium_version=%s\n' "$cronet_digest" "$size" "$chromium"
    (cd "$root" && shasum -a 256 ops/runtime/sing-box/cronet.sh)
  } > "$work/companion.txt"
}
