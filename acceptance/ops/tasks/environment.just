# Install pinned Go and just locally under build/, without changing system configuration.
[group('environment')]
bootstrap:
    bash ops/environment/bootstrap.sh

# Verify the active development tools and show their versions.
[group('environment')]
doctor:
    @test "$(go env GOVERSION)" = "go$(awk '$1 == "go" { print $2 }' go.mod)"
    @test "$(just --version)" = 'just 1.58.0'
    @go version
    @just --version
    @command -v ssh curl tar shasum
    @go env GOTOOLCHAIN GOPATH GOMODCACHE GOCACHE

# Reconcile pinned Go module/tool dependencies after an intentional go.mod edit.
[group('environment')]
deps-tidy:
    GOFLAGS= go mod tidy

# Build the pinned security scanner for the host, independent of scan target platforms.
[group('environment')]
audit-tool:
    @mkdir -p build/tools/bin
    @bash ops/quality/check.sh security-audit env GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" go build -trimpath -o build/tools/bin/govulncheck golang.org/x/vuln/cmd/govulncheck
