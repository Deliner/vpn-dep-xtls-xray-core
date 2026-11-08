# Prepare a reviewable CI branch in the existing maintained Xray fork from clean committed sources.
[group('acceptance')]
acceptance-prepare output suite='protocols':
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/prepare.sh "$1" "$2"

# Read the owned CI repository's Actions policy, workflows and runs for one exact preparation.
[group('acceptance')]
acceptance-status directory:
    #!/usr/bin/env bash
    set -euo pipefail
    source ops/testing/acceptance/identity.sh
    acceptance_identity
    acceptance_directory "$1"
    gh api --hostname github.com "repos/$acceptance_repo/actions/permissions" --jq .
    gh api --hostname github.com --paginate "repos/$acceptance_repo/actions/workflows?per_page=100" --jq '.workflows[]|{id,state,path}'
    gh api --hostname github.com "repos/$acceptance_repo/actions/runs?head_sha=$acceptance_commit&per_page=100" --jq '.workflow_runs[]|{id,status,conclusion,head_sha,html_url}'

# MUTATES GITHUB: enable only the prepared acceptance run after disabling inherited workflows, then push without force.
[group('acceptance')]
acceptance-publish directory:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/publish.sh "$1"

# Build and audit both maintained engines, then execute every profile on the requested native Linux CPU.
[group('acceptance')]
acceptance-run arch:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/run.sh "$1"

# Build a pinned Cronet Linux target on a native AMD64 runner, retaining source/toolchain/linkage evidence.
[group('acceptance')]
acceptance-cronet-run arch:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/cronet.sh "$1"

# Verify native CPU and execute the compiled Cronet regression from a checksum-verified CI artifact.
[group('acceptance')]
acceptance-cronet-verify arch candidate:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/cronet-verify.sh "$1" "$2"

# Bind verified Go candidates to the new native library and test Naive H2/H3 TCP/UDP on native Linux.
[group('acceptance')]
acceptance-cronet-payload arch candidate output:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/cronet-payload.sh "$1" "$2" "$3"

# Verify native Linux all-protocol TUN and foreground/systemd recovery with race instrumentation.
[group('acceptance')]
acceptance-client-run arch:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/client/run.sh "$1"

# Download a completed matching CI run, restore captured Actions policy and independently verify its evidence.
[group('acceptance')]
acceptance-collect directory run_id:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/collect.sh "$1" "$2"
