# Import one trusted generated route into a new private client profile, without changing networking.
[group('client')]
client-import bundle route output:
    go run ./cmd/nhop-client import --bundle "$1" --route "$2" --output "$3"

# Prepare Linux TUN, DNS and kill-switch files for an explicit dedicated service UID; no activation.
[group('client')]
client-plan profile output engine_uid ipv6='tunnel':
    go run ./cmd/nhop-client plan --profile "$1" --output "$2" --engine-uid "$3" --ipv6 "$4"

# Test actual nftables egress denial only in nested namespaces inside the owned Linux VM.
[group('client')]
client-kernel-test:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/kernel.sh

# Exercise real Linux TUN TCP/UDP/IPv6/DNS and leak denial after engine/front-end failure in the owned VM.
[group('client')]
client-tun-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/tun.sh "$1" "$2" "$3"

# Verify per-TUN system DNS configuration and restoration with a private resolved/D-Bus in the owned VM.
[group('client')]
client-resolver-test:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/resolver.sh

# Exercise production session start/stop and engine-failure cleanup with real TUN/DNS/nftables in the owned VM.
[group('client')]
client-session-test xray singbox:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/session.sh "$1" "$2"
