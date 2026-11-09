#!/usr/bin/env bash
# Local replacements lose their upstream identity in govulncheck. Scan that
# identity too, conservatively, using a separate modfile and untouched cache.
candidate_tuic_audit_upstream() {
  local module=github.com/sagernet/sing-quic
  local version=v0.7.2-0.20261020082049-7e0a5869a5f4
  local name
  for name in tuic-upstream.mod tuic-upstream.sum; do
    [[ ! -L "$reports/$name" && ( ! -e "$reports/$name" || -f "$reports/$name" ) ]] || return 2
  done
  [[ "$(go -C "$candidate/source" list -m -f '{{.Version}}' "$module")" == "$version" ]] || return 2
  [[ "$(go -C "$candidate/source" list -m -f '{{.Replace.Dir}}' "$module")" == "$candidate/source/nhop-dependencies/sing-quic" ]] || return 2
  cp "$candidate/source/go.mod" "$reports/tuic-upstream.mod" || return 2
  cp "$candidate/source/go.sum" "$reports/tuic-upstream.sum" || return 2
  go mod edit "-modfile=$reports/tuic-upstream.mod" "-dropreplace=$module@$version" || return 2
  [[ "$(GOFLAGS="-modfile=$reports/tuic-upstream.mod -mod=readonly" go -C "$candidate/source" list -m -f '{{if .Replace}}replaced{{end}}' "$module")" == '' ]] || return 2
  printf '\nConservative upstream identity scan: %s@%s\n' "$module" "$version"
  GOFLAGS="-modfile=$reports/tuic-upstream.mod -mod=readonly" \
    "$scanner" -C "$candidate/source" -tags "$tags" -show traces,verbose "$package"
}
