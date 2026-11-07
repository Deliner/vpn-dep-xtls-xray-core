#!/usr/bin/env bash
set -euo pipefail
umask 077
source "$(dirname "${BASH_SOURCE[0]}")/lab.sh"
client_lab_begin
client_lab_run policy
