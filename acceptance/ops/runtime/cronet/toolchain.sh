#!/usr/bin/env bash
# Configure only already reviewed toolchain inputs; no downloader is executed here.
native_prepare_tools() {
  local clang gn ninja clang_directory
  clang="$(native_pin_input clang-mac-arm64)"
  gn="$(native_pin_input gn-mac-arm64)"
  ninja="$(native_pin_input ninja-mac)"
  clang_directory="$src/third_party/llvm-build/Release+Asserts"
  mkdir -p "$clang_directory" "$src/gn/out" "$work/tools"
  tar -xf "$clang" -C "$clang_directory"
  unzip -p "$gn" gn > "$src/gn/out/gn"
  unzip -p "$ninja" ninja > "$work/tools/ninja-real"
  chmod 0755 "$src/gn/out/gn" "$work/tools/ninja-real"
  printf 'llvmorg-24-init-3796-g20e97c4b-27\n' > "$clang_directory/cr_build_revision"
  printf 'git_revision:150a9d6ba0aa7f407aa4feeabc5f03ce9aa7e04b\n' > "$src/gn/out/cr_build_revision"
  printf '#!/usr/bin/env bash\nexec %q -j4 "$@"\n' "$work/tools/ninja-real" > "$work/tools/ninja"
  printf '#!/usr/bin/env bash\nprintf "Unplanned native input download refused\\n" >&2\nexit 2\n' > "$work/tools/curl"
  chmod 0755 "$work/tools/ninja" "$work/tools/curl"
}

native_prepare_target() {
  local profile component profile_file sysroot destination
  component=pgo-linux
  profile_file="$src/chrome/build/linux.pgo.txt"
  if [[ "$target" == darwin/arm64 ]]; then
    component=pgo-mac-arm64
    profile_file="$src/chrome/build/mac-arm.pgo.txt"
  fi
  profile="$(native_pin_input "$component")"
  profile_name="$(cat "$profile_file")"
  [[ "$profile_name" =~ ^[A-Za-z0-9._-]+\.profdata$ ]] || return 2
  [[ "$(awk -F '\t' -v component="$component" '$1 == component {print $3}' "$recipe/inputs.tsv")" == */"$profile_name" ]] || return 2
  mkdir -p "$src/chrome/build/pgo_profiles"
  cp "$profile" "$src/chrome/build/pgo_profiles/$profile_name"
  if [[ "$target" == linux/* ]]; then
    sysroot="$(native_pin_input "sysroot-linux-${target#*/}")"
    destination="$src/out/sysroot-build/bullseye/bullseye_${target#*/}_staging"
    mkdir -p "$destination"
    tar -xf "$sysroot" -C "$destination"
    [[ -d "$destination/lib" ]] || return 2
  fi
}
