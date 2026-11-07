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

# Build an isolated Xray probe that logs HTTP/3 queue drops; synthetic lab traffic only.
[group('runtime')]
masque-probe-build candidate output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/masque-probe.sh "$1" "$2"

# Download an exact native toolchain input for checksum review without extracting or executing it.
[group('runtime')]
cronet-review-input component:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/cronet/review-input.sh "$1"

# Build a native Cronet candidate from exact owned sources and reviewed toolchain inputs on macOS ARM64.
[group('runtime')]
cronet-build target output:
    @bash ops/quality/check.sh static-analysis bash ops/runtime/cronet/build.sh "$1" "$2"
