package acceptance

const fakeGit = `#!/usr/bin/env bash
set -eu
if [[ " $* " == *" push --porcelain "* ]]; then
  printf 'push\n' >> "$CI_TEST_TRACE"
  exit 43
fi
exec "$CI_TEST_GIT" "$@"
`

const fakeGitHub = `#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$CI_TEST_TRACE"
case "$*" in
  'api --hostname github.com user --jq '*)
    printf 'Deliner\t26311467\n' ;;
  'api --hostname github.com repos/Deliner/vpn-dep-xtls-xray-core --jq '*)
    printf '1449369505\t26311467\tDeliner/vpn-dep-xtls-xray-core\tfalse\ttrue\n' ;;
  'api --hostname github.com repos/Deliner/vpn-dep-xtls-xray-core/actions/permissions --jq '*)
    printf '%s\t%s\n' "${CI_TEST_ENABLED:-false}" "${CI_TEST_POLICY:-all}" ;;
  'api --hostname github.com --paginate '*'/actions/workflows?per_page=100 --jq '*)
    printf '101\tactive\t.github/workflows/upstream.yml\n102\tdisabled_manually\t.github/workflows/other.yml\n'
    if [[ "${CI_TEST_REGISTERED:-0}" == 1 ]]; then printf '303\tactive\t.github/workflows/nhop-acceptance.yml\n'; fi ;;
  'api --hostname github.com --method PUT '*'/workflows/'*'/disable')
    [[ "${CI_TEST_DISABLE_FAILURE:-0}" == 0 ]] || exit 17 ;;
  'api --hostname github.com --method PUT '*'/actions/permissions -F enabled=true') ;;
  'api --hostname github.com --method PUT '*'/actions/permissions --input '*) ;;
  'api --hostname github.com '*'/workflows/101 --jq '*)
    printf 'disabled_manually\t.github/workflows/upstream.yml\n' ;;
  'api --hostname github.com '*'/workflows/102 --jq '*)
    printf 'disabled_fork\t.github/workflows/other.yml\n' ;;
  'api --hostname github.com --method PUT '*'/workflows/101/enable') ;;
  'api --hostname github.com --method PUT '*'/workflows/303/enable') ;;
  *) printf 'Unexpected test API request\n' >&2; exit 2 ;;
esac
`
