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
