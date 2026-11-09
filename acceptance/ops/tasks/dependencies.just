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

# Refresh an owned checkout's Go locks after reviewed module edits, then verify authenticated module content.
[group('dependencies')]
dependency-lock directory:
    #!/usr/bin/env bash
    set -euo pipefail
    directory="$(cd "$1" && pwd -P)"
    root="$(pwd -P)"
    case "$directory/" in "$root/build/dependencies/checkouts/"*) ;; *) exit 2 ;; esac
    [[ "$(git -C "$directory" rev-parse --show-toplevel)" == "$directory" ]] || exit 2
    export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
    export GOPRIVATE= GONOPROXY= GONOSUMDB=
    GOFLAGS=-mod=mod bash ops/quality/check.sh static-analysis go -C "$directory" mod tidy
    bash ops/quality/check.sh static-analysis go -C "$directory" mod verify

# Format explicitly selected Go files in an owned dependency checkout with the pinned toolchain.
[group('dependencies')]
dependency-format directory +files:
    #!/usr/bin/env bash
    set -euo pipefail
    directory="$(cd "$1" && pwd -P)"
    root="$(pwd -P)"
    case "$directory/" in "$root/build/dependencies/checkouts/"*) ;; *) exit 2 ;; esac
    [[ "$(git -C "$directory" rev-parse --show-toplevel)" == "$directory" ]] || exit 2
    shift
    selected=()
    for file in "$@"; do
      [[ "$file" == *.go && -f "$directory/$file" && ! -L "$directory/$file" ]] || exit 2
      resolved="$(cd "$(dirname "$directory/$file")" && pwd -P)/$(basename "$file")"
      case "$resolved" in "$directory/"*) ;; *) exit 2 ;; esac
      selected+=("$resolved")
    done
    bash ops/quality/check.sh formatting gofmt -w "${selected[@]}"

# MUTATES GITHUB: publish a committed maintenance branch without force or default-branch changes.
[group('dependencies')]
dependency-push repository:
    @bash ops/quality/check.sh static-analysis bash ops/dependencies/push.sh "$1"

# Regenerate the QUIC sender and ACK-handler mocks from their pinned tool and maintained interfaces.
[group('dependencies')]
dependency-quic-mocks:
    #!/usr/bin/env bash
    set -euo pipefail
    directory="$PWD/build/dependencies/checkouts/vpn-dep-apernet-quic-go"
    case "$(git -C "$directory" remote get-url origin)" in
      https://github.com/Deliner/vpn-dep-apernet-quic-go|https://github.com/Deliner/vpn-dep-apernet-quic-go.git) ;;
      *) exit 2 ;;
    esac
    GOWORK=off GOFLAGS=-mod=readonly bash ops/quality/check.sh static-analysis go -C "$directory" generate -run mock_sender_test.go mockgen.go
    GOWORK=off GOFLAGS=-mod=readonly bash ops/quality/check.sh static-analysis go -C "$directory/internal/mocks" generate -run ackhandler/sent_packet_handler.go mockgen.go
