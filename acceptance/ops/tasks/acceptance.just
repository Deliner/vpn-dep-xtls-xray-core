# Prepare a reviewable CI branch in the existing maintained Xray fork from clean committed sources.
[group('acceptance')]
acceptance-prepare output:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/prepare.sh "$1"

# MUTATES GITHUB: enable only the prepared acceptance run after disabling inherited workflows, then push without force.
[group('acceptance')]
acceptance-publish directory:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/publish.sh "$1"

# Build and audit both maintained engines, then execute every profile on the requested native Linux CPU.
[group('acceptance')]
acceptance-run arch:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/run.sh "$1"

# Download a completed matching CI run and verify its native payload reports before restoring disabled Actions.
[group('acceptance')]
acceptance-collect directory run_id:
    @bash ops/quality/check.sh static-analysis bash ops/testing/acceptance/collect.sh "$1" "$2"
