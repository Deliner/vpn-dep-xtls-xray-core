# Run Go tests; optionally limit to one package pattern.
[group('testing')]
test package='./...':
    go test "$1"

# Run a selected regression uncached with its evidence output.
[group('testing')]
test-case package pattern:
    @bash ops/quality/check.sh static-analysis go test -count=1 -v -run "$2" "$1"

# Run the race detector (requires a C toolchain).
[group('testing')]
test-race:
    CGO_ENABLED=1 go test -race ./...

# Measure Go benchmarks with allocation counts and repeated samples.
[group('testing')]
bench package='./...' pattern='.' duration='1s' count='3':
    go test -run '^$' -bench "$2" -benchtime "$3" -count "$4" -benchmem "$1"

# Download the pinned native Xray and sing-box only for local payload testing.
[group('testing')]
payload-setup:
    bash ops/testing/fetch-xray.sh
    bash ops/testing/fetch-sing-box.sh

# Check real client probes, TCP/UDP, concurrent profiles and failures with the race detector.
[group('testing')]
payload xray='build/xray-test/xray' singbox='build/sing-box-test/sing-box' pattern='Test' report='build/reports/concurrent':
    @just payload-runtimes "$1" "$2" "$1" "$2" "$3" "$4"

# Select separate client/server binaries and optional updated nodes for a payload run.
[group('testing')]
payload-runtimes server_xray server_singbox client_xray client_singbox pattern='Test' report='build/reports/concurrent' updated_xray='' updated_singbox='' updated_nodes='':
    bash ops/testing/payload.sh "$@"

# Test official/candidate clients and servers in both directions plus two N-hop rollout stages.
[group('testing')]
payload-compatibility candidate_xray candidate_singbox report='build/reports/compatibility':
    bash ops/testing/compatibility.sh "$1" "$2" "$3"

# Require complete reports with the expected producer OS/CPU and ordered node addresses.
[group('testing')]
payload-report report hops='1,3' since='0' platform='' addresses='':
    go run ./cmd/payloadcheck "$1" "$2" "$3" "$4" "$5"

# Download and validate pinned Linux engine binaries and Naive's libcronet on both CPUs.
[group('testing')]
runtime-artifacts:
    CHECK_RUNTIME_ARTIFACTS=1 go test -count=1 -v ./internal/quality/runtimeaudit

# Run all implemented profiles together with TCP/UDP origin barriers; save JSON reports.
[group('testing')]
payload-concurrent xray='build/xray-test/xray' singbox='build/sing-box-test/sing-box' report='build/reports/concurrent':
    #!/usr/bin/env bash
    set -euo pipefail
    started="$(date +%s)"
    just payload "$1" "$2" '^TestAllProtocolsConcurrent$' "$3"
    just payload-report "$3" '1,3' "$started"

# Run every profile concurrently across IPv6 and mixed IPv4/IPv6 node chains.
[group('testing')]
payload-ipv6 xray='build/xray-test/xray' singbox='build/sing-box-test/sing-box' report='build/reports/ipv6':
    #!/usr/bin/env bash
    set -euo pipefail
    started="$(date +%s)"
    NHOP_IPV6_ACCEPTANCE=1 just payload "$1" "$2" '^TestIPv6ProtocolMatrix$' "$3"
    just payload-report "$3/ipv6-single" '1' "$started" '' '::1'
    just payload-report "$3/ipv6-chain" '3' "$started" '' '::1'
    just payload-report "$3/mixed-6-4-6" '3' "$started" '' '::1,127.0.0.1,::1'
    just payload-report "$3/mixed-4-6-4" '3' "$started" '' '127.0.0.1,::1,127.0.0.1'
