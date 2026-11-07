# Format Go sources and the task catalog.
[group('development')]
fmt:
    gofmt -w cmd internal
    just --fmt
    @while IFS= read -r -d '' task; do just --justfile "$task" --fmt; done < <(find ops/tasks -type f -name '*.just' -print0)

# Check all structural constraints, repair instructions and go vet.
[group('development')]
lint:
    @mkdir -p build/tools/bin
    @bash ops/quality/check.sh static-analysis go build -o build/tools/bin/repolint ./cmd/repolint
    build/tools/bin/repolint
    @bash ops/quality/check.sh formatting just --fmt --check
    @while IFS= read -r -d '' task; do bash ops/quality/check.sh formatting just --justfile "$task" --fmt --check; done < <(find ops/tasks -type f -name '*.just' -print0)
    @bash ops/quality/check.sh static-analysis go vet ./...
    @while IFS= read -r -d '' script; do bash ops/quality/check.sh static-analysis bash -n "$script"; done < <(find ops -type f -name '*.sh' -print0)

# Read the full semantic repair workflow for a lint rule.
[group('development')]
explain rule:
    @go run ./cmd/repolint --explain "$1"

# Compile controller, local manifest checker and both Linux installer architectures.
[group('development')]
build:
    mkdir -p build/bin
    go build -trimpath -buildvcs=false -o build/bin/nhop ./cmd/nhop
    go build -trimpath -buildvcs=false -o build/bin/nhop-node ./cmd/nhop-node
    go build -trimpath -buildvcs=false -o build/bin/nhop-client ./cmd/nhop-client
    GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o build/bin/nhop-node-linux-amd64 ./cmd/nhop-node
    GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o build/bin/nhop-node-linux-arm64 ./cmd/nhop-node
    GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o build/bin/nhop-client-linux-amd64 ./cmd/nhop-client
    GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o build/bin/nhop-client-linux-arm64 ./cmd/nhop-client

# Complete local acceptance check; no SSH, installation or firewall changes.
[group('development')]
check: lint docs-check test build

# Scan known Go vulnerabilities for every supported OS/CPU using the pinned scanner.
[group('development')]
audit: audit-tool
    @bash ops/quality/check.sh security-audit bash ops/quality/audit.sh build/tools/bin/govulncheck build/reports/security

# Verify pinned Linux runtime archives and audit Go symbols/dependencies without executing them.
[group('development')]
audit-runtime report='build/reports/security-runtime': audit-tool
    @mkdir -p "$1"
    @RUNTIME_AUDIT_SCANNER="$(pwd)/build/tools/bin/govulncheck" RUNTIME_AUDIT_REPORT="$(cd "$1" && pwd)" CHECK_RUNTIME_ARTIFACTS=1 bash ops/quality/check.sh security-audit go test -count=1 -v -run '^TestPinnedLinuxArchives$' ./internal/quality/runtimeaudit

# Triage runtime findings against a verified upstream source tree and its build tags.
[group('development')]
audit-upstream directory packages='./...' tags='': audit-tool
    @bash ops/quality/check.sh security-audit build/tools/bin/govulncheck -C "$1" -tags "$3" -show traces,verbose "$2"
