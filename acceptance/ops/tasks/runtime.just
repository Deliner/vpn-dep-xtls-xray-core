# Refresh reviewed dependency locks for an isolated runtime candidate; never changes release pins.
[group('runtime')]
candidate-lock engine:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/candidate.sh lock "$1"

# Build an explicitly labelled candidate from verified source and locked modules into a new build/ directory.
[group('runtime')]
candidate-build engine os arch output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/candidate.sh build "$1" "$2" "$3" "$4"

# Audit one candidate's source and binary, preserving both scanner results.
[group('runtime')]
candidate-audit directory: audit-tool
    @bash ops/quality/check.sh security-audit bash ops/runtime/audit-candidate.sh "$1"

# Verify that two completed candidates retain identical source inventories and executable bytes.
[group('runtime')]
candidate-compare first second:
    bash ops/runtime/compare.sh "$1" "$2"

# Observe Xray queues, send-size failures and inner TCP boundaries; synthetic lab traffic only.
[group('runtime')]
masque-probe-build candidate output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/masque-probe.sh "$1" "$2"

# Reproduce PMTU probe losses incorrectly reaching extended congestion control, then race-test a private patch.
[group('runtime')]
quic-mtu-congestion-test output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/xray/quic/regression.sh "$1"

# Run both verified QUIC variants and complete ACK/congestion packages natively in the owned Linux VM.
[group('runtime')]
quic-mtu-congestion-linux-test regression output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/xray/quic/native.sh "$1" "$2"

# Build a control or patched Xray with identical embedded QUIC sources; experimental, without release pin changes.
[group('runtime')]
quic-mtu-candidate base regression variant output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/xray/quic/candidate.sh "$1" "$2" "$3" "$4"

# Download an exact native toolchain input for checksum review without extracting or executing it.
[group('runtime')]
cronet-review-input component:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/cronet/review-input.sh "$1"

# Build a native Cronet candidate from exact owned sources and reviewed inputs on Linux AMD64 or macOS ARM64.
[group('runtime')]
cronet-build target output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/cronet/build.sh "$1" "$2"

# Execute Cronet's overlong HTTP range regression with a verified native build in the owned Linux VM.
[group('runtime')]
cronet-range-test candidate:
    @bash ops/quality/check.sh static-analysis bash ops/testing/cronet/range.sh "$1"

# Reproduce the zlib dictionary bound defect and verify a private patch under ASan/UBSan; no network changes.
[group('runtime')]
cronet-zlib-window-test output:
    @bash ops/quality/check.sh static-analysis bash ops/testing/cronet/zlib/run.sh "$1"

# Check the exact vendored ICU UTF subset and SuperFastHash with bounded inputs and ASan/UBSan; no network changes.
[group('runtime')]
cronet-component-bounds-test output:
    @bash ops/quality/check.sh static-analysis bash ops/testing/cronet/components/run.sh "$1"

# Reproduce TUIC's missing QUIC shutdown on the exact locked graph, then race-test the local patch.
[group('runtime')]
tuic-shutdown-test output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/sing-box/tuic/regression.sh "$1"

# Repeat the verified TUIC shutdown regression natively inside the owned Fleet Linux VM.
[group('runtime')]
tuic-shutdown-linux-test directory:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/sing-box/tuic/native.sh "$1"

# Race-test the exact patched TUIC sources embedded in a verified candidate on the current native host.
[group('runtime')]
tuic-candidate-test candidate output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/sing-box/tuic/candidate.sh "$1" "$2"
