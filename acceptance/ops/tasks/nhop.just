# Validate YAML without contacting servers.
[group('nhop')]
validate topology:
    go run ./cmd/nhop validate "$1"

# Print topology and exit-first installation order without secrets.
[group('nhop')]
plan topology:
    go run ./cmd/nhop plan "$1"

# Build installers and create private per-node and client configurations.
[group('nhop')]
render topology output='build/nhop': build
    build/bin/nhop render "$1" --output "$2"

# Validate one node manifest locally without installing it.
[group('nhop')]
node-check config:
    go run ./cmd/nhop-node --config "$1" --check

# MUTATES VPS: upload installers over SSH; downstream first, up to jobs independent nodes.
[group('nhop')]
deploy bundle='build/nhop' jobs='1':
    go run ./cmd/nhop deploy "$1" --jobs "$2"

# Verify configured SSH ports and public exit IP using the selected client runtime.
[group('nhop')]
verify bundle='build/nhop' xray='build/xray-test/xray' ip_url='https://api.ipify.org' singbox='build/sing-box-test/sing-box':
    go run ./cmd/nhop verify "$1" --xray "$2" --ip-url "$3" --sing-box "$4"

# Run a generated client profile with the pinned local runtime.
[group('nhop')]
client profile='build/nhop/clients/main.json' xray='build/xray-test/xray' singbox='build/sing-box-test/sing-box':
    go run ./cmd/nhop client "$1" --xray "$2" --sing-box "$3"

# List implemented profiles and research-only items with prerequisites.
[group('nhop')]
protocols:
    go run ./cmd/nhop protocols
