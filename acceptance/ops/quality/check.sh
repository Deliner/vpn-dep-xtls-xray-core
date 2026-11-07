#!/usr/bin/env bash
set -euo pipefail

if (($# < 2)); then
  printf 'usage: check.sh RULE COMMAND [ARGS...]\n' >&2
  exit 2
fi
rule="$1"
shift
[[ "$rule" =~ ^[a-z][a-z0-9-]*$ ]] || exit 2
quality_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_name="$rule"
[[ "$rule" != security-audit ]] || skill_name="static-analysis/security-audit"
skill_relative="ops/quality/skills/$skill_name/SKILL.md"
skill_file="$quality_dir/skills/$skill_name/SKILL.md"
common_file="$quality_dir/skills/refactor/SKILL.md"
if [[ ! -f "$skill_file" || ! -f "$common_file" ]]; then
  printf 'Required repair skill missing: %s (and refactor/SKILL.md)\n' "$skill_relative" >&2
  exit 2
fi

if "$@"; then
  exit 0
else
  status=$?
fi
{
  printf '\n[%s] Проверка %s завершилась с кодом %s.\n' "$rule" "$1" "$status"
  printf '  Для исправления используй skill `%s`: %s\n' "$rule" "$skill_relative"
  printf '  Прочитать: just explain %s\n\n' "$rule"
  cat "$common_file" "$skill_file"
} >&2
exit "$status"
