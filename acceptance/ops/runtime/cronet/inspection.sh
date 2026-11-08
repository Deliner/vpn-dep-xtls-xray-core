#!/usr/bin/env bash
# Build evidence belongs to the completed native tree, never to source guesses.
native_inspect() {
  local final=libcronet.so response
  if [[ "$native_os" != linux ]]; then final=obj/components/cronet/libcronet_static.a; fi
  response="$native_output/$final.rsp"
  [[ -f "$response" && -s "$response" && ! -L "$response" ]] || { printf 'Final linker response file was not retained: %s\n' "$final" >&2; return 2; }
  mkdir "$work/inspection" || return 1
  cp "$native_output/args.gn" "$work/inspection/args.gn" || return 1
  cp "$src/components/cronet/native/include/cronet_c.h" "$work/inspection/cronet_c.h" || return 1
  (cd "$src" && "$src/gn/out/gn" desc "$native_output" //components/cronet:cronet deps --all) > "$work/inspection/dependencies.txt" || return 1
  (cd "$src" && "$work/tools/ninja-real" -C "$native_output" -t commands cronet) > "$work/inspection/commands.txt" || return 1
  "$work/tools/ninja-real" -C "$native_output" -t deps > "$work/inspection/header-dependencies.txt" || return 1
  grep -q ': #deps [1-9][0-9]*,.*(VALID)' "$work/inspection/header-dependencies.txt" || { printf 'No valid compiler header dependency records retained\n' >&2; return 2; }
  "$work/tools/ninja-real" -C "$native_output" -t query "$final" > "$work/inspection/link-query.txt" || return 1
  [[ -s "$work/inspection/link-query.txt" ]] || return 2
  cp "$response" "$work/inspection/link-response.txt" || return 1
  (cd "$native_output" && find . -type f -name '*.rsp' -print0 | LC_ALL=C sort -z | COPYFILE_DISABLE=1 tar --no-xattrs --null -T - -cf "$work/inspection/response-files.tar") || return 1
  printf '%s\n' "$final" > "$work/inspection/link-target.txt"
}
