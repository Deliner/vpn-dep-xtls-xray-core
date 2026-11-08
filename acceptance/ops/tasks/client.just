# Import one trusted generated route into a new private client profile, without changing networking.
[group('client')]
client-import bundle route output:
    go run ./cmd/nhop-client import --bundle "$1" --route "$2" --output "$3"

# Prepare Linux TUN, DNS and kill-switch files for an explicit dedicated service UID; no activation.
[group('client')]
client-plan profile output engine_uid ipv6='tunnel':
    go run ./cmd/nhop-client plan --profile "$1" --output "$2" --engine-uid "$3" --ipv6 "$4"

# Render a manual-start Linux systemd unit without installing it or changing host networking.
[group('client')]
client-service-plan binary profile runtimes output ipv6='tunnel':
    go run ./cmd/nhop-client service-plan --binary "$1" --profile "$2" --runtimes "$3" --output "$4" --ipv6 "$5"

# MUTATES LINUX FILES: install a verified private client generation and inactive manual-start unit; never enables boot.
[group('client')]
client-service-install profile runtimes ipv6='tunnel':
    ./build/bin/nhop-client service-install --profile "$1" --runtimes "$2" --ipv6 "$3"

# MUTATES LINUX FILES: unregister the verified inactive unit, retaining snapshots and recovery state.
[group('client')]
client-service-remove:
    ./build/bin/nhop-client service-remove

# MUTATES LINUX FILES: register early protection and client boot start; no immediate VPN/network activation.
[group('client')]
client-service-enable network_manager='':
    ./build/bin/nhop-client service-enable --network-manager "$1"

# MUTATES LINUX FILES: remove verified boot dependencies without releasing the current firewall policy.
[group('client')]
client-service-disable:
    ./build/bin/nhop-client service-disable

# Verify actual CLI installation/update/interruption/rollback/unregister with systemd only in the owned Linux VM.
[group('client')]
client-install-test xray singbox:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/run.sh "$1" "$2"

# Run the actually installed Linux service through private TUN/DNS, process failures and guarded recovery in the owned VM.
[group('client')]
client-installed-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/managed.sh "$1" "$2" "$3"

# MUTATES LINUX CLIENT NETWORK: stay in foreground with root-only control and fail-closed cancellation.
[group('client')]
client-start profile runtimes ipv6='tunnel':
    ./build/bin/nhop-client start --profile "$1" --runtimes "$2" --ipv6 "$3"

# MUTATES LINUX CLIENT NETWORK: start/resume protected intent and exit on failure for a service supervisor.
[group('client')]
client-service-run profile runtimes ipv6='tunnel':
    ./build/bin/nhop-client service-run --profile "$1" --runtimes "$2" --ipv6 "$3"

# Linux-root service preflight: verify/create private controller state without changing networking.
[group('client')]
client-service-prepare:
    ./build/bin/nhop-client service-prepare

# MUTATES LINUX CLIENT NETWORK: retain offline egress protection before any session or network configuration.
[group('client')]
client-boot-protect:
    ./build/bin/nhop-client boot-protect

# Test retained early protection and gap-free handoff to the real TUN client only in the owned Linux VM.
[group('client')]
client-boot-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/boot.sh "$1" "$2" "$3"

# Verify actual systemd boot ordering, guard failure and retained protection only in the owned Linux VM.
[group('client')]
client-boot-order-test:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/order.sh

# Prepare separate client/exit VMs for real boot acceptance using the pinned local lab tool and image.
[group('client')]
client-reboot-lab-setup:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/reboot/setup.sh

# Start only the two checkout-owned client reboot VMs; leaves Mac networking unchanged.
[group('client')]
client-reboot-lab-start:
    bash ops/testing/client/install/reboot/vm.sh start

# Show both client reboot VM states without starting them.
[group('client')]
client-reboot-lab-status:
    bash ops/testing/client/install/reboot/vm.sh status

# Stop only the two checkout-owned client reboot VMs, retaining disks and evidence.
[group('client')]
client-reboot-lab-stop:
    bash ops/testing/client/install/reboot/vm.sh stop

# Execute a command only in boot-client or boot-exit using private SSH metadata.
[group('client')]
client-reboot-lab-shell node +args:
    bash ops/testing/client/install/reboot/vm.sh shell "$@"

# Reboot the owned Linux client VM with a separate exit, early leak probe and local recovery timer.
[group('client')]
client-reboot-test xray singbox:
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/install/reboot/run.sh "$1" "$2"

# Show credential-free local session status; unavailable controllers are explicitly labelled.
[group('client')]
client-diagnostics:
    ./build/bin/nhop-client diagnostics

# MUTATES LINUX CLIENT NETWORK: reconnect a failed session while retaining the same kill switch.
[group('client')]
client-reconnect:
    ./build/bin/nhop-client reconnect

# MUTATES LINUX CLIENT NETWORK: resume the protected journal in foreground after controller death.
[group('client')]
client-resume:
    ./build/bin/nhop-client resume

# MUTATES LINUX CLIENT NETWORK: explicitly stop or recover the owned session and restore ordinary networking.
[group('client')]
client-stop:
    ./build/bin/nhop-client stop

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

# Run the public Linux CLI, private NSS account creation, failures and controller-crash recovery in the owned VM.
[group('client')]
client-public-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/public.sh "$1" "$2" "$3"

# Verify real systemd restart, protected intent and continuous leak denial only in the owned Linux VM.
[group('client')]
client-service-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/systemd.sh "$1" "$2" "$3"

# Verify every generated protocol through the real Linux CLI/TUN over single and mixed three-hop routes in the owned VM.
[group('client')]
client-protocol-test xray singbox ipv6='tunnel':
    @bash ops/quality/check.sh static-analysis bash ops/testing/client/protocols.sh "$1" "$2" "$3"
