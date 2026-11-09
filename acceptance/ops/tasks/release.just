# Export an allowlisted source archive without secrets, Git or generated files.
[group('release')]
export output='build/vpn-script-public.tar.gz':
    bash ops/release/export-public.sh "$1"

# Refresh the source checksum manifest from the same allowlisted export.
[group('release')]
checksums: export
    tar -xOf build/vpn-script-public.tar.gz vpn-script/ops/release/SHA256SUMS > ops/release/SHA256SUMS

# Recompute the maintained research inventory after moves or document edits.
[group('release')]
research-inventory:
    go run ./cmd/repodocs inventory

# Check maintained Markdown links and the research inventory.
[group('release')]
docs-check:
    @mkdir -p build/tools/bin
    @bash ops/quality/check.sh static-analysis go build -o build/tools/bin/repodocs ./cmd/repodocs
    build/tools/bin/repodocs check

# Package a reviewed Linux Xray candidate only when its bytes and complete native evidence match the release policy.
[group('release')]
runtime-release-package arch candidate native_report output:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/package.sh "$1" "$2" "$3" "$4"

# Assemble both reviewed Xray packages into a new immutable publication directory.
[group('release')]
runtime-release-stage amd64 arm64 output:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/stage.sh "$1" "$2" "$3"

# Reconstruct reviewed source inventories from both archives and verify every publication input.
[group('release')]
runtime-release-verify directory:
    @bash ops/quality/check.sh static-analysis go run ./cmd/runtimepack verify ops/release/runtime/xray.json "$1"

# MUTATES GITHUB: publish a maintained Xray prerelease only after verifying a downloaded draft; never overwrite assets.
[group('release')]
runtime-release-publish directory:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/publish.sh "$1"

# Verify reviewed Cronet library bytes and complete native ABI/Naive/zlib evidence on both CPUs.
[group('release')]
cronet-release-verify collection:
    @bash ops/quality/check.sh static-analysis go run ./cmd/runtimepack cronet-verify ops/release/runtime/cronet.json "$1"

# Create immutable Cronet delivery archives after verifying both native targets.
[group("release")]
cronet-release-package collection output:
    @bash ops/quality/check.sh static-analysis go run ./cmd/runtimepack cronet-package ops/release/runtime/cronet.json "$1" "$2"

# Verify downloaded Cronet archives against the reviewed native library and evidence hashes.
[group("release")]
cronet-release-package-verify directory:
    @bash ops/quality/check.sh static-analysis go run ./cmd/runtimepack cronet-package-verify ops/release/runtime/cronet.json "$1"

# Package complete reviewed Cronet source inputs and their license texts without altering checkouts.
[group('release')]
cronet-release-sources output:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/cronet-sources.sh "$1"

# Assemble reviewed Cronet binaries, full source archives and license notices into eleven assets.
[group('release')]
cronet-release-stage binaries sources output:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/cronet/stage.sh "$1" "$2" "$3"

# Verify every downloaded Cronet publication asset against reviewed native inputs.
[group('release')]
cronet-release-verify-stage directory:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/cronet/verify.sh "$1"

# MUTATES GITHUB: publish the owned Cronet prerelease after verifying downloaded draft assets; never overwrite.
[group('release')]
cronet-release-publish directory:
    @bash ops/quality/check.sh static-analysis bash ops/release/runtime/cronet/publish.sh "$1"
