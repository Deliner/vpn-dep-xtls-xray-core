#!/usr/bin/env bash
# Only committed allowlisted project sources enter the acceptance branch.
acceptance_copy_sources() {
  local project="$1" checkout="$2" output="$3" tracked archived
  mkdir "$checkout/acceptance"
  git -C "$project" archive --format=tar HEAD go.mod go.sum justfile cmd internal \
    ops examples/nhop/topology.yaml > "$output/source.tar"
  tar -xf "$output/source.tar" -C "$checkout/acceptance"
  printf 'project_commit=%s\nsource_archive_sha256=%s\n' "$(git -C "$project" rev-parse HEAD)" \
    "$(shasum -a 256 "$output/source.tar" | awk '{print $1}')" > "$checkout/acceptance/source.txt"
  # The parent Xray repository ignores every directory named xray and coverage.go.
  # Force applies only to this fresh subtree extracted from our tracked source archive.
  git -C "$checkout" add --force -- acceptance
  archived="$(tar -tf "$output/source.tar" | sed '/\/$/d' | LC_ALL=C sort)"
  tracked="$(git -C "$checkout" ls-files acceptance | sed 's|^acceptance/||; /^source.txt$/d' | LC_ALL=C sort)"
  [[ "$archived" == "$tracked" ]] || { printf 'Committed acceptance source inventory differs from reviewed archive.\n' >&2; return 2; }
}
