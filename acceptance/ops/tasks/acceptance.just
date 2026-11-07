# Prepare a reviewable CI branch in the existing maintained Xray fork from clean committed sources.
[group('acceptance')]
acceptance-prepare output suite='protocols':
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/prepare.sh "$1" "$2"

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

# Download a completed matching CI run, restore captured Actions policy and independently verify its evidence.
[group('acceptance')]
acceptance-collect directory run_id:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/collect.sh "$1" "$2"
