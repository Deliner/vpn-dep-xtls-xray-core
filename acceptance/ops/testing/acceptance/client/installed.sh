#!/usr/bin/env bash
set -euo pipefail
[[ $# == 2 ]] || exit 2
source "$(dirname "${BASH_SOURCE[0]}")/verify.sh"
source "$(dirname "${BASH_SOURCE[0]}")/verify-installed.sh"
client_verify_installed "$1" "$2"
