# Record every selected Go module, replacement and available source origin in all maintained module roots.
[group('dependencies')]
dependency-inventory:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork inventory

# Plan GitHub repository forks from recorded source origins; retain unresolved origins explicitly.
[group('dependencies')]
dependency-plan:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork plan

# Resolve old module origins directly from VCS in an isolated metadata cache, preserving the active fork plan.
[group('dependencies')]
dependency-origins offset='0' limit='10':
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork origins "$1" "$2"

# Merge complete verified origin checkpoints into a separate inventory, preserving the current fork plan.
[group('dependencies')]
dependency-enrich:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork enrich

# Describe the enriched source graph for audit; does not authorize repository creation.
[group('dependencies')]
dependency-resolved-plan:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork resolved-plan

# Describe newly discovered origins and exact verified mirrors without creating repositories.
[group('dependencies')]
dependency-additional-plan:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork additional-plan

# Resolve an owned Go module at an exact commit with sum.golang.org checks, outside maintained module roots.
[group('dependencies')]
dependency-module module revision:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork owned-module "$1" "$2"

# Plan deletion only for this session's recorded excess repositories, preserving the explicit maintenance scope.
[group('dependencies')]
dependency-cleanup-plan:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork cleanup-plan

# MUTATES GITHUB: delete a bounded batch from the exact reviewed cleanup plan; verify each absence.
[group('dependencies')]
dependency-cleanup-apply plan_sha offset limit='25':
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork cleanup-apply "$1" "$2" "$3"

# Re-read the owner's complete repository inventory and verify removals plus every protected ID.
[group('dependencies')]
dependency-cleanup-verify plan_sha:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork cleanup-verify "$1"

# Record every Chromium component descriptor from a clean, exactly pinned native source fork.
[group('dependencies')]
dependency-native directory revision:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork native "$1" "$2"

# Fetch a fresh fork source archive and compare every member and executable bit with its exact clean Git checkout.
[group('dependencies')]
dependency-source directory revision:
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork source "$1" "$2"

# MUTATES GITHUB: create or verify only a fork explicitly listed in ops/dependencies/forks.tsv.
[group('dependencies')]
dependency-fork upstream owner='Deliner':
    @bash ops/quality/check.sh static-analysis go run ./cmd/depfork fork "$1" "$2"

# Fetch one reviewed commit from a maintained fork into a fresh local checkout.
[group('dependencies')]
dependency-checkout repository revision:
    @bash ops/quality/check.sh static-analysis bash ops/dependencies/checkout.sh "$1" "$2"

# Fetch the exact Cronet wrapper Git tree with native binary modules outside the sparse worktree.
[group('dependencies')]
dependency-cronet-checkout revision:
    @bash ops/quality/check.sh static-analysis bash ops/dependencies/checkout.sh Deliner/vpn-dep-sagernet-cronet-go "$1" cronet-source

# Resolve a public fork ref to a Git object before reviewing and checking it out.
[group('dependencies')]
dependency-ref repository ref:
    gh api --hostname github.com "repos/$1/commits/$2" --jq '{sha: .sha, url: .html_url}'

# Run selected upstream regression tests with the project's toolchain and race detector.
[group('dependencies')]
dependency-test directory packages pattern='Test' tags='':
    @CGO_ENABLED=1 bash ops/quality/check.sh static-analysis go -C "$1" test -count=1 -race -v -tags "$4" -run "$3" "$2"

# MUTATES GITHUB: publish a committed maintenance branch without force or default-branch changes.
[group('dependencies')]
dependency-push repository:
    @bash ops/quality/check.sh static-analysis bash ops/dependencies/push.sh "$1"
