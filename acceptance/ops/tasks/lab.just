# Download pinned Lima/Ubuntu and create a private Linux lab on macOS.
[group('lab')]
lab-setup:
    bash ops/testing/linux/setup.sh

# Start only this checkout's Linux VM; no shared host directories or VPN forwards.
[group('lab')]
lab-start:
    bash ops/testing/linux/vm.sh start

# Show the private Linux VM state.
[group('lab')]
lab-status:
    bash ops/testing/linux/vm.sh status

# Stop only this checkout's Linux VM, keeping its disk for the next run.
[group('lab')]
lab-stop:
    bash ops/testing/linux/vm.sh stop

# Execute an explicit diagnostic command inside this checkout's Linux VM.
[group('lab')]
lab-shell +command:
    bash ops/testing/linux/vm.sh shell "$@"

# Package a standalone Linux payload test and two verified candidate runtimes.
[group('lab')]
lab-package arch xray singbox output:
    bash ops/testing/linux/package.sh "$1" "$2" "$3" "$4"

# Run the complete standalone payload suite in the private Linux VM and copy evidence back.
[group('lab')]
lab-payload xray singbox:
    bash ops/testing/linux/run.sh payload "$1" "$2"

# Package systemd acceptance, optionally including a trusted previous installer and its SHA-256.
[group('lab')]
lab-installer-package arch output previous='' sha256='':
    bash ops/testing/linux/installer/package.sh "$1" "$2" "$3" "$4"

# Apply/update/roll back a real VPN service only inside the private Linux VM and clean it up.
[group('lab')]
lab-installer previous='' sha256='':
    bash ops/testing/linux/run.sh installer "$1" "$2"

# Exhaust automatic ports, verify planned listeners and exclusive SOCKS UDP ports in an isolated VM namespace.
[group('lab')]
lab-socks-ports run xray:
    bash ops/testing/linux/ports.sh "$1" "$2"

# Verify fail-closed routing and repeated UDP/443 denial against a real isolated low-port canary.
[group('lab')]
lab-routing run xray singbox:
    bash ops/testing/linux/routing.sh "$1" "$2" "$3"

# Repeat synthetic protocol restarts with an instrumented MASQUE probe and retain every process log.
[group('lab')]
lab-masque xray singbox probe repeats='20' debug='0':
    bash ops/testing/diagnostics/masque.sh "$1" "$2" "$3" "$4" "$5"

# Exercise MASQUE H3 under netem loss/delay/MTU, burst load and restarts in private PID/network namespaces.
[group('lab')]
lab-masque-netem probe cycles='3' workers='8' packets='32':
    @bash ops/quality/check.sh static-analysis bash ops/testing/diagnostics/netem/lab.sh "$1" "$2" "$3" "$4"

# Verify controller SSH/SCP, literal paths and host-key refusal against the owned VM.
[group('lab')]
lab-ssh:
    bash ops/testing/ssh.sh
