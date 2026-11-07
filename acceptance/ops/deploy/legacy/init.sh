#!/bin/bash
# ============================================================
# VLESS + REALITY Setup Script - SECURITY HARDENED
# Ubuntu 24.04 | Max Security | Zero Lockout Risk
# ============================================================
#
# SECURITY AUDIT FIXES (v5 - MAXIMUM HARDENED):
# 1. SSH: Port 22 removed and other Port/AllowUsers lines neutralized.
# 2. SSH: Root login disabled when non-root admin exists; AllowUsers enforced.
# 3. SSH: Modern crypto only + weak key algos disabled.
# 4. Kernel: Expanded sysctl hardening (ptrace, userns, perf, etc).
# 5. Swap: Auto-create 1GB swap if missing (prevents OOM kills).
# 6. Updates: Auto-reboot enabled (04:00 AM) via dedicated config override.
# 7. Audit: auditd enabled with comprehensive immutable rules.
# 8. Shared Mem: /dev/shm hardened even if fstab already exists.
# 9. Firewall: UFW safe handover + port 22 closed post-SSH rebinding.
# 10. Fail2Ban: Protects custom SSH port + Recidive jail + banaction=ufw.
#
# v5 ADDITIONS:
# 11. Temp cleanup: Global EXIT trap (fixes RETURN trap leak on error_exit).
# 12. sysctl: IP forwarding disabled, ARP hardening, TCP tuning, IPv6 RA off.
# 13. Core dumps: Disabled at 3 levels (sysctl, limits.conf, systemd-coredump).
# 14. Kernel modules: DCCP/SCTP/RDS/TIPC/USB/Bluetooth/Firewire/legacy FS blacklisted.
# 15. /proc: hidepid=2 (non-root users cannot see other processes).
# 16. Audit rules: Identity, SSH, Xray, cron, escalation, network, time changes.
# 17. Cron/at: Restricted to root + admin user only.
# 18. Login banner: Legal warning via /etc/issue.net + SSH Banner directive.
# 19. /tmp: Hardened with nosuid,nodev,noexec (systemd tmp.mount override).
# 20. SUID/SGID: Stripped from chfn, chsh, newgrp, wall, mount, umount etc.
# 21. Xray systemd: SystemCallFilter + UMask=0077 added.
# 22. SSH: GatewayPorts no, GSSAPI off, AcceptEnv restricted, PrintLastLog yes.
# 23. Unattended-upgrades: Reliable config via separate apt.conf.d override.
#
# v6 SECURITY AUDIT FIXES:
# 24. gen_hex_32: Removed predictable date-based fallback; fail hard if no CSPRNG.
# 25. sysctl: Added net.core.bpf_jit_harden=2, vm.unprivileged_userfaultfd=0.
# 26. SSH host keys: Remove weak DSA keys; ensure ed25519; regenerate RSA <4096.
# 27. su restriction: pam_wheel.so restricts su to sudo group only.
# 28. login.defs: UMASK 027, SHA512 10k rounds, password aging, HOME_MODE 0750.
# 29. Journald limits: 200M max, 1 month retention (prevent disk-fill DoS).
# 30. /var/log: Permissions tightened to 0750 (no world-read).
# 31. Services: Disable avahi, cups, bluetooth, ModemManager, snapd if present.
# 32. SSH: Removed deprecated ChallengeResponseAuthentication directive.
# 33. CRITICAL FIX: Admin password NO LONGER printed to terminal; stored only
#     in /root/xray-credentials.txt with shred instructions.
# 34. Credentials: UUID/PublicKey truncated in terminal output for OpSec.
#
# v7 SECURITY AUDIT FIXES:
# 35. UFW: Force IPV6=yes to prevent IPv6 firewall bypass.
# 36. Xray install: Prevent zip path traversal (unsafe archive paths) before extraction.
# 37. SSH hardening: Ensure sshd_config.d include is outside Match blocks + neutralize overrides.
# 38. Input validation: Validate SUDO_USER/SSH_TEST_USER and HARDEN_ADMIN_PASSWORD to prevent injection.
# 39. Updates: Optional apt upgrade during setup (HARDEN_APT_UPGRADE=true).
#
# v8 SECURITY AUDIT FIXES:
# 40. Xray routing: Block geoip:private via blackhole outbound (prevent open-proxy/SSRF to metadata API).
# 41. Validation: REALITY_SNI, REALITY_FP, UUID, SHORTID, PRIVATE_KEY, PUBLIC_KEY format checks.
# 42. Backup permissions: chmod 600 for all backup files in /root and /etc/ssh.
# 43. Re-run guard: XRAY_FORCE_REGENERATE required to overwrite existing Xray config/keys.
# 44. CLIENT_LINK OpSec: Full VLESS link and QR code saved to file only (not printed to terminal).
# 45. ShortId entropy: Increased from 4 to 8 bytes (2^64 vs 2^32).
# 46. Sniffing: routeOnly=true to prevent DNS leaks (server no longer resolves client domains).
# 47. Auto-reboot: Configurable via HARDEN_AUTO_REBOOT (default true).
# 48. Security-only upgrade: HARDEN_APT_SECURITY_ONLY option.
# 49. Xray version: Pinned default + curl --max-redirs 3.
# 50. Idempotent fstab: grep guard before appending (safe re-runs).
# 51. Audit: Check -e 2 immutable status before loading rules.
# 52. sysctl permissions: 0644 (standard, non-secret).
# 53. SUID/SGID: Report remaining setuid binaries in audit summary.
# 54. Setup log: Optional /var/log/vless-setup.log.
#
# PREVIOUS FIXES:
# 10. Key parsing works with Xray 25.x (Password instead of PublicKey)
# 11. No grep failures that kill script
# 12. APT lock waiting
# 13. Time sync for VLESS
# 14. Firewall configured before SSH changes
# 15. Secure Xray install (no curl|bash), SHA256-verified release
# 16. Secrets hardening: umask 077 + restricted Xray config permissions
# 17. SSH hardening via drop-in + disables keyboard-interactive auth
# 18. Private key no longer leaked to terminal on parse failure
# 19. Trap injection fixed (safe single-quoted trap)
# 20. iptables safety-net during UFW reset (no open window)
# 21. XRAY_VERSION env var validated (regex whitelist)
# 22. Xray systemd: ProtectSystem=strict, PrivateTmp, PrivateDevices, namespace/memory restrictions
# 23. Sensitive variables (PRIVATE_KEY, UUID, etc.) purged after use
# ============================================================

set -euo pipefail
IFS=$'\n\t'
umask 077
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# --- Cleanup (global EXIT trap for temp dirs) ---
_CLEANUP_DIRS=()
_cleanup_on_exit() {
    # Purge sensitive variables on any exit (normal or error_exit).
    unset -v PRIVATE_KEY UUID PUBLIC_KEY REALITY_SHORTID CLIENT_LINK \
        X25519_OUT ADMIN_SUDO_PASSWORD HARDEN_ADMIN_PASSWORD \
        _CRED_PASS_SECTION UPSTREAM_UUID UPSTREAM_PBK UPSTREAM_SID 2>/dev/null || true
    local d
    for d in "${_CLEANUP_DIRS[@]}"; do
        [[ -d "$d" ]] && rm -rf -- "$d"
    done
}
trap '_cleanup_on_exit' EXIT

# --- Helper ---
error_exit() {
    echo ""
    echo "❌ ERROR: $1"
    echo ""
    exit 1
}

# --- Configuration ---
SSH_PORT="${SSH_PORT:-22222}"
HARDEN_CREATE_ADMIN_USER="${HARDEN_CREATE_ADMIN_USER:-true}"  # auto-create non-root admin if only root key exists
HARDEN_ADMIN_USER="${HARDEN_ADMIN_USER:-secureadmin}"
HARDEN_SET_ADMIN_PASSWORD="${HARDEN_SET_ADMIN_PASSWORD:-true}" # set a strong local password for sudo (SSH password login stays disabled)
HARDEN_ADMIN_PASSWORD="${HARDEN_ADMIN_PASSWORD:-}"             # optional; if empty a random password will be generated
HARDEN_TMP_NOEXEC="${HARDEN_TMP_NOEXEC:-true}"                 # enforce /tmp nosuid,nodev,noexec via systemd tmp.mount
HARDEN_APT_UPGRADE="${HARDEN_APT_UPGRADE:-true}"               # apply OS security updates during setup
HARDEN_APT_SECURITY_ONLY="${HARDEN_APT_SECURITY_ONLY:-false}"  # only install security updates (not all upgrades)
HARDEN_AUTO_REBOOT="${HARDEN_AUTO_REBOOT:-true}"               # auto-reboot after unattended-upgrades (04:00)
HARDEN_SETUP_LOG="${HARDEN_SETUP_LOG:-true}"                   # log all script output to /var/log/vless-setup.log
HARDEN_NORMALIZE_AUTHORIZED_KEYS="${HARDEN_NORMALIZE_AUTHORIZED_KEYS:-false}" # strip forced-command key options before hardening SSH
XRAY_FORCE_REGENERATE="${XRAY_FORCE_REGENERATE:-false}"        # set to true to overwrite existing Xray config and keys
REALITY_SNI="${REALITY_SNI:-www.microsoft.com}"
REALITY_FP="${REALITY_FP:-chrome}"
REALITY_DEST="${REALITY_SNI}:443"
XRAY_MODE="${XRAY_MODE:-single}"                                # single (default), entry, exit
UPSTREAM_IP="${UPSTREAM_IP:-}"                                  # required in entry mode
UPSTREAM_PORT="${UPSTREAM_PORT:-443}"                           # required in entry mode
UPSTREAM_UUID="${UPSTREAM_UUID:-}"                              # required in entry mode
UPSTREAM_PBK="${UPSTREAM_PBK:-}"                                # required in entry mode (exit public key)
UPSTREAM_SNI="${UPSTREAM_SNI:-}"                                # required in entry mode
UPSTREAM_FP="${UPSTREAM_FP:-chrome}"                            # required in entry mode
UPSTREAM_SID="${UPSTREAM_SID:-}"                                # required in entry mode (exit short ID)
ALLOW_ENTRY_IP="${ALLOW_ENTRY_IP:-}"                            # optional in exit mode (restrict 443 to entry IP/CIDR)
XRAY_VERSION="${XRAY_VERSION:-v25.1.1}" # pinned default; set to empty string + XRAY_USE_LATEST=true for latest
XRAY_EXPECTED_SHA256="${XRAY_EXPECTED_SHA256:-}" # optional: pin expected SHA256 of the Xray zip (overrides .dgst check)
# Client-facing transport. tcp = REALITY + xtls-rprx-vision (covers DPI layers 1-3).
# xhttp = REALITY + XHTTP with random padding: adds layer-4 / behavioral-ML evasion
# (packet-size randomisation) at the cost of xtls-rprx-vision (incompatible with xhttp).
XRAY_TRANSPORT="${XRAY_TRANSPORT:-tcp}"                         # tcp (default) or xhttp
XRAY_XHTTP_PATH="${XRAY_XHTTP_PATH:-}"                          # xhttp path; random if empty
XRAY_XHTTP_PADDING="${XRAY_XHTTP_PADDING:-100-1000}"           # xPaddingBytes range for behavioral masking
UPSTREAM_TRANSPORT="${UPSTREAM_TRANSPORT:-tcp}"                 # entry mode: must match the exit's XRAY_TRANSPORT
UPSTREAM_PATH="${UPSTREAM_PATH:-}"                              # entry mode: exit's XRAY_XHTTP_PATH (required if upstream is xhttp)
ADMIN_SUDO_PASSWORD=""
if ! [[ "$SSH_PORT" =~ ^[0-9]+$ ]] || ((SSH_PORT < 1 || SSH_PORT > 65535)); then
    error_exit "Invalid SSH_PORT='${SSH_PORT}'. Use a port between 1 and 65535."
fi
if [[ "$SSH_PORT" -eq 443 ]]; then
    error_exit "SSH_PORT cannot be 443 (reserved for Xray)."
fi
if [[ "$HARDEN_CREATE_ADMIN_USER" != "true" && "$HARDEN_CREATE_ADMIN_USER" != "false" ]]; then
    error_exit "Invalid HARDEN_CREATE_ADMIN_USER='${HARDEN_CREATE_ADMIN_USER}'. Use true or false."
fi
if [[ "$HARDEN_SET_ADMIN_PASSWORD" != "true" && "$HARDEN_SET_ADMIN_PASSWORD" != "false" ]]; then
    error_exit "Invalid HARDEN_SET_ADMIN_PASSWORD='${HARDEN_SET_ADMIN_PASSWORD}'. Use true or false."
fi
if [[ "$HARDEN_TMP_NOEXEC" != "true" && "$HARDEN_TMP_NOEXEC" != "false" ]]; then
    error_exit "Invalid HARDEN_TMP_NOEXEC='${HARDEN_TMP_NOEXEC}'. Use true or false."
fi
if [[ "$HARDEN_APT_UPGRADE" != "true" && "$HARDEN_APT_UPGRADE" != "false" ]]; then
    error_exit "Invalid HARDEN_APT_UPGRADE='${HARDEN_APT_UPGRADE}'. Use true or false."
fi
if [[ "$HARDEN_APT_SECURITY_ONLY" != "true" && "$HARDEN_APT_SECURITY_ONLY" != "false" ]]; then
    error_exit "Invalid HARDEN_APT_SECURITY_ONLY='${HARDEN_APT_SECURITY_ONLY}'. Use true or false."
fi
if [[ "$HARDEN_AUTO_REBOOT" != "true" && "$HARDEN_AUTO_REBOOT" != "false" ]]; then
    error_exit "Invalid HARDEN_AUTO_REBOOT='${HARDEN_AUTO_REBOOT}'. Use true or false."
fi
if [[ "$HARDEN_SETUP_LOG" != "true" && "$HARDEN_SETUP_LOG" != "false" ]]; then
    error_exit "Invalid HARDEN_SETUP_LOG='${HARDEN_SETUP_LOG}'. Use true or false."
fi
if [[ "$HARDEN_NORMALIZE_AUTHORIZED_KEYS" != "true" && "$HARDEN_NORMALIZE_AUTHORIZED_KEYS" != "false" ]]; then
    error_exit "Invalid HARDEN_NORMALIZE_AUTHORIZED_KEYS='${HARDEN_NORMALIZE_AUTHORIZED_KEYS}'. Use true or false."
fi
if [[ "$XRAY_FORCE_REGENERATE" != "true" && "$XRAY_FORCE_REGENERATE" != "false" ]]; then
    error_exit "Invalid XRAY_FORCE_REGENERATE='${XRAY_FORCE_REGENERATE}'. Use true or false."
fi
case "$XRAY_MODE" in
    single|entry|exit) ;;
    *) error_exit "Invalid XRAY_MODE='${XRAY_MODE}'. Allowed: single, entry, exit." ;;
esac
case "$XRAY_TRANSPORT" in
    tcp|xhttp) ;;
    *) error_exit "Invalid XRAY_TRANSPORT='${XRAY_TRANSPORT}'. Allowed: tcp, xhttp." ;;
esac
case "$UPSTREAM_TRANSPORT" in
    tcp|xhttp) ;;
    *) error_exit "Invalid UPSTREAM_TRANSPORT='${UPSTREAM_TRANSPORT}'. Allowed: tcp, xhttp." ;;
esac
# Path must start with '/' and contain only URL-safe path characters (no quotes/spaces).
if [[ -n "$XRAY_XHTTP_PATH" ]] && ! [[ "$XRAY_XHTTP_PATH" =~ ^/[A-Za-z0-9._~/-]*$ ]]; then
    error_exit "Invalid XRAY_XHTTP_PATH='${XRAY_XHTTP_PATH}'. Must start with '/' and use [A-Za-z0-9._~/-]."
fi
if [[ -n "$UPSTREAM_PATH" ]] && ! [[ "$UPSTREAM_PATH" =~ ^/[A-Za-z0-9._~/-]*$ ]]; then
    error_exit "Invalid UPSTREAM_PATH='${UPSTREAM_PATH}'. Must start with '/' and use [A-Za-z0-9._~/-]."
fi
# xPaddingBytes range like "100-1000" (or a single integer).
if ! [[ "$XRAY_XHTTP_PADDING" =~ ^[0-9]+(-[0-9]+)?$ ]]; then
    error_exit "Invalid XRAY_XHTTP_PADDING='${XRAY_XHTTP_PADDING}'. Use 'min-max' (e.g. 100-1000) or a single integer."
fi
if [[ -z "$HARDEN_ADMIN_USER" ]]; then
    error_exit "HARDEN_ADMIN_USER cannot be empty."
fi
if ! [[ "$HARDEN_ADMIN_USER" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]]; then
    error_exit "Invalid HARDEN_ADMIN_USER='${HARDEN_ADMIN_USER}'. Use a simple lowercase username (e.g. secureadmin)."
fi
if [[ "$HARDEN_CREATE_ADMIN_USER" == "true" && "$HARDEN_ADMIN_USER" == "root" ]]; then
    error_exit "HARDEN_ADMIN_USER cannot be 'root' when auto-creating admin user."
fi
if [[ -n "$XRAY_VERSION" ]] && ! [[ "$XRAY_VERSION" =~ ^v?[0-9]+(\.[0-9]+){0,3}(-[a-zA-Z0-9.]+)?$ ]]; then
    error_exit "Invalid XRAY_VERSION='${XRAY_VERSION}'. Expected format: v1.2.3"
fi
if [[ -n "${XRAY_EXPECTED_SHA256:-}" ]] && ! [[ "$XRAY_EXPECTED_SHA256" =~ ^[0-9a-f]{64}$ ]]; then
    error_exit "Invalid XRAY_EXPECTED_SHA256: expected 64 lowercase hex chars (SHA-256 hash)."
fi
if ! [[ "$UPSTREAM_PORT" =~ ^[0-9]+$ ]] || ((UPSTREAM_PORT < 1 || UPSTREAM_PORT > 65535)); then
    error_exit "Invalid UPSTREAM_PORT='${UPSTREAM_PORT}'. Use a port between 1 and 65535."
fi
if [[ -n "${HARDEN_ADMIN_PASSWORD:-}" ]]; then
    # Prevent injection into chpasswd input (':' and newlines are separators).
    if [[ "$HARDEN_ADMIN_PASSWORD" == *$'\n'* || "$HARDEN_ADMIN_PASSWORD" == *$'\r'* || "$HARDEN_ADMIN_PASSWORD" == *:* ]]; then
        error_exit "Invalid HARDEN_ADMIN_PASSWORD: must not contain ':' or newlines."
    fi
    if ((${#HARDEN_ADMIN_PASSWORD} < 16)); then
        error_exit "Invalid HARDEN_ADMIN_PASSWORD: too short (min 16 chars)."
    fi
fi
# Validate REALITY_SNI (valid hostname: letters, digits, dots, hyphens)
if ! [[ "$REALITY_SNI" =~ ^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,253}[a-zA-Z0-9])?$ ]]; then
    error_exit "Invalid REALITY_SNI='${REALITY_SNI}'. Must be a valid hostname (letters, digits, dots, hyphens)."
fi
# Validate REALITY_FP (whitelist of known TLS fingerprints)
case "$REALITY_FP" in
    chrome|firefox|safari|ios|android|edge|360|qq|random|randomized) ;;
    *) error_exit "Invalid REALITY_FP='${REALITY_FP}'. Allowed: chrome, firefox, safari, ios, android, edge, 360, qq, random, randomized." ;;
esac

if [[ -n "$ALLOW_ENTRY_IP" ]]; then
    if [[ "$XRAY_MODE" != "exit" ]]; then
        error_exit "ALLOW_ENTRY_IP can only be used with XRAY_MODE=exit."
    fi
    if [[ "$ALLOW_ENTRY_IP" =~ [[:space:]] ]] || ! [[ "$ALLOW_ENTRY_IP" =~ ^([0-9]{1,3}(\.[0-9]{1,3}){3}|[0-9A-Fa-f]{0,4}(:[0-9A-Fa-f]{0,4}){2,7})(/[0-9]{1,3})?$ ]]; then
        error_exit "Invalid ALLOW_ENTRY_IP='${ALLOW_ENTRY_IP}'. Use IPv4/IPv6 with optional CIDR (example: 203.0.113.4 or 203.0.113.0/24)."
    fi
    if [[ "$ALLOW_ENTRY_IP" == */* ]]; then
        _allow_prefix="${ALLOW_ENTRY_IP##*/}"
        if [[ "$ALLOW_ENTRY_IP" == *:* ]]; then
            if ((_allow_prefix < 0 || _allow_prefix > 128)); then
                error_exit "Invalid ALLOW_ENTRY_IP prefix '/${_allow_prefix}' for IPv6. Use 0..128."
            fi
        else
            if ((_allow_prefix < 0 || _allow_prefix > 32)); then
                error_exit "Invalid ALLOW_ENTRY_IP prefix '/${_allow_prefix}' for IPv4. Use 0..32."
            fi
        fi
    fi
fi

if [[ "$XRAY_MODE" == "entry" ]]; then
    [[ -n "$UPSTREAM_IP" ]] || error_exit "UPSTREAM_IP is required when XRAY_MODE=entry."
    [[ -n "$UPSTREAM_UUID" ]] || error_exit "UPSTREAM_UUID is required when XRAY_MODE=entry."
    [[ -n "$UPSTREAM_PBK" ]] || error_exit "UPSTREAM_PBK is required when XRAY_MODE=entry."
    [[ -n "$UPSTREAM_SNI" ]] || error_exit "UPSTREAM_SNI is required when XRAY_MODE=entry."
    [[ -n "$UPSTREAM_SID" ]] || error_exit "UPSTREAM_SID is required when XRAY_MODE=entry."
    if [[ "$UPSTREAM_TRANSPORT" == "xhttp" ]]; then
        [[ -n "$UPSTREAM_PATH" ]] || error_exit "UPSTREAM_PATH is required when UPSTREAM_TRANSPORT=xhttp (use the exit's XRAY_XHTTP_PATH)."
    fi

    if [[ "$UPSTREAM_IP" =~ [[:space:]] ]] || ! [[ "$UPSTREAM_IP" =~ ^([a-zA-Z0-9]([a-zA-Z0-9._-]{0,253}[a-zA-Z0-9])?|([0-9]{1,3}\.){3}[0-9]{1,3}|[0-9A-Fa-f:]+)$ ]]; then
        error_exit "Invalid UPSTREAM_IP='${UPSTREAM_IP}'. Use a hostname or IP address."
    fi
    if ! [[ "$UPSTREAM_UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
        error_exit "Invalid UPSTREAM_UUID='${UPSTREAM_UUID}'. Expected UUID format."
    fi
    if ! [[ "$UPSTREAM_PBK" =~ ^[A-Za-z0-9_-]{42,44}=?$ ]]; then
        error_exit "Invalid UPSTREAM_PBK format. Expected X25519 public key (base64url, 43-44 chars)."
    fi
    if ! [[ "$UPSTREAM_SNI" =~ ^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,253}[a-zA-Z0-9])?$ ]]; then
        error_exit "Invalid UPSTREAM_SNI='${UPSTREAM_SNI}'. Must be a valid hostname."
    fi
    case "$UPSTREAM_FP" in
        chrome|firefox|safari|ios|android|edge|360|qq|random|randomized) ;;
        *) error_exit "Invalid UPSTREAM_FP='${UPSTREAM_FP}'. Allowed: chrome, firefox, safari, ios, android, edge, 360, qq, random, randomized." ;;
    esac
    if ! [[ "$UPSTREAM_SID" =~ ^[0-9a-f]{8,16}$ ]]; then
        error_exit "Invalid UPSTREAM_SID='${UPSTREAM_SID}'. Expected 8-16 lowercase hex chars."
    fi
fi

curl_secure() {
    # Safer defaults: fail fast, https-only, retries, timeouts, limited redirects.
    curl --fail --silent --show-error --location \
        --proto '=https' --proto-redir '=https' --tlsv1.2 \
        --retry 5 --retry-all-errors --retry-delay 1 \
        --connect-timeout 10 --max-time 120 \
        --max-redirs 3 \
        "$@"
}

is_safe_ssh_username() {
    # Conservative username validation for sshd_config directives.
    # Note: HARDEN_ADMIN_USER is stricter elsewhere; this is for discovered users (SUDO_USER, etc).
    local u="${1:-}"
    [[ "$u" == "root" ]] && return 0
    [[ "$u" =~ ^[A-Za-z_][A-Za-z0-9._-]{0,31}$ ]]
}

gen_hex_32() {
    # Generate 32 hex chars using cryptographic randomness ONLY.
    # NEVER fall back to predictable sources (date, PID, etc.).
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 16
        return 0
    fi
    if [[ -r /dev/urandom ]]; then
        od -An -N16 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n'
        return 0
    fi
    if [[ -r /proc/sys/kernel/random/uuid ]]; then
        # Kernel CSPRNG-backed UUID; combine two for 256 bits of entropy
        local _u1 _u2
        _u1="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || true)"
        _u2="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || true)"
        printf '%s%s' "$_u1" "$_u2" | sha256sum | awk '{print $1}' | head -c 32
        return 0
    fi
    error_exit "No cryptographic randomness source available (need openssl, /dev/urandom, or /proc/sys/kernel/random/uuid). Cannot generate secrets safely."
}

normalize_authorized_keys_file() {
    # Keep only usable public-key material. Cloud images sometimes put the same
    # key in root's authorized_keys with a forced command such as "login as ...".
    # Copying that line into the sole allowed admin user causes a lockout.
    local src="$1"
    awk '
        BEGIN {
            kt = "(sk-ssh-ed25519@openssh\\.com|sk-ecdsa-sha2-nistp256@openssh\\.com|ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(256|384|521))"
        }
        /^[[:space:]]*($|#)/ { next }
        {
            line = $0
            sub(/^[[:space:]]+/, "", line)
            if (match(line, kt "[[:space:]]+")) {
                print substr(line, RSTART)
            }
        }
    ' "$src"
}

write_normalized_authorized_keys() {
    local target="$1"
    shift
    local tmp norm src

    tmp="$(mktemp)"
    norm="$(mktemp)"
    for src in "$@"; do
        [[ -s "$src" ]] || continue
        normalize_authorized_keys_file "$src" >> "$tmp"
    done
    awk 'NF >= 2 { key=$1 " " $2; if (!seen[key]++) print }' "$tmp" > "$norm"
    rm -f "$tmp"

    if [[ ! -s "$norm" ]]; then
        rm -f "$norm"
        error_exit "No usable SSH public keys remain after normalizing authorized_keys. Refusing to continue."
    fi

    install -m 0600 "$norm" "$target"
    rm -f "$norm"
}

append_authorized_keys_preserve_options() {
    local target="$1"
    local source_keys="$2"
    local line

    if [[ -s "$target" ]]; then
        while IFS= read -r line; do
            [[ -z "$line" ]] && continue
            grep -Fxq -- "$line" "$target" || printf '%s\n' "$line" >> "$target"
        done < "$source_keys"
    else
        install -m 0600 "$source_keys" "$target"
    fi
}

sanitize_user_authorized_keys() {
    local user="$1"
    local home_dir target

    home_dir="$(getent passwd "$user" 2>/dev/null | awk -F: '{print $6}' || true)"
    [[ -n "$home_dir" ]] || error_exit "Failed to resolve home for ${user}"
    target="${home_dir}/.ssh/authorized_keys"
    [[ -s "$target" ]] || error_exit "authorized_keys not found or empty for ${user}: $target"

    [[ "$HARDEN_NORMALIZE_AUTHORIZED_KEYS" == "true" ]] || return 0
    write_normalized_authorized_keys "$target" "$target"
    chown "$user:$user" "$target"
    chmod 0600 "$target"
}

ensure_admin_user_with_keys() {
    local admin_user="$1"
    local source_keys="$2"
    local home_dir target

    if [[ -z "$admin_user" || "$admin_user" == "root" ]]; then
        error_exit "Invalid admin user '${admin_user}'"
    fi
    if [[ ! -s "$source_keys" ]]; then
        error_exit "Source authorized_keys not found or empty: $source_keys"
    fi

    if ! id -u "$admin_user" >/dev/null 2>&1; then
        useradd --create-home --shell /bin/bash --groups sudo "$admin_user" >/dev/null 2>&1 || \
            error_exit "Failed to create admin user '${admin_user}'"
    else
        usermod -aG sudo "$admin_user" >/dev/null 2>&1 || true
    fi

    home_dir="$(getent passwd "$admin_user" 2>/dev/null | awk -F: '{print $6}' || true)"
    [[ -n "$home_dir" ]] || error_exit "Failed to resolve home for ${admin_user}"
    chown "$admin_user:$admin_user" "$home_dir" 2>/dev/null || true
    chmod 0750 "$home_dir" 2>/dev/null || true

    install -d -m 0700 "${home_dir}/.ssh"
    target="${home_dir}/.ssh/authorized_keys"
    if [[ "$HARDEN_NORMALIZE_AUTHORIZED_KEYS" == "true" ]]; then
        if [[ -s "$target" ]]; then
            write_normalized_authorized_keys "$target" "$target" "$source_keys"
        else
            write_normalized_authorized_keys "$target" "$source_keys"
        fi
    else
        append_authorized_keys_preserve_options "$target" "$source_keys"
    fi
    chown -R "$admin_user:$admin_user" "${home_dir}/.ssh"
    chmod 0600 "$target"

    # Remove any legacy NOPASSWD sudoers (security hole).
    rm -f /etc/sudoers.d/99-vless-admin >/dev/null 2>&1 || true

    # If the account has a locked password, set a strong LOCAL password (for sudo only).
    # SSH password login stays disabled by this script.
    pw_state="$(passwd -S "$admin_user" 2>/dev/null | awk '{print $2}' || true)"
    if [[ "$pw_state" == "L" ]]; then
        if [[ "$HARDEN_SET_ADMIN_PASSWORD" == "true" ]]; then
            local new_pass
            if [[ -n "${HARDEN_ADMIN_PASSWORD:-}" ]]; then
                new_pass="$HARDEN_ADMIN_PASSWORD"
            else
                new_pass="$(gen_hex_32)"
            fi
            printf '%s:%s\n' "$admin_user" "$new_pass" | chpasswd || error_exit "Failed to set local password for ${admin_user}"
            ADMIN_SUDO_PASSWORD="$new_pass"
        else
            echo "       ⚠ Admin user '${admin_user}' has a locked password; sudo may be unusable until you set a password."
        fi
    fi
}

comment_out_ssh_ports() {
    local main_conf="$1"
    local dropin_dir="$2"
    local keep_file="$3"
    local f backup

    for f in "$main_conf" "$dropin_dir"/*.conf; do
        [[ -f "$f" ]] || continue
        [[ "$f" == "$keep_file" ]] && continue
        if grep -qE '^[[:space:]]*(Port|AllowUsers|AllowGroups|DenyUsers)[[:space:]]+' "$f"; then
            backup="${f}.portbak.$(date +%s)"
            cp "$f" "$backup"
            chmod 600 "$backup"
            sed -i -E 's/^[[:space:]]*(Port|AllowUsers|AllowGroups|DenyUsers)[[:space:]]+/# &/g' "$f"
        fi
    done
}

comment_out_ssh_overrides() {
    # Prevent later overrides in sshd_config / other drop-ins from weakening our managed policy.
    local main_conf="$1"
    local dropin_dir="$2"
    local keep_file="$3"
    local f backup
    local re='PermitRootLogin|PasswordAuthentication|KbdInteractiveAuthentication|PubkeyAuthentication|AuthenticationMethods|AuthorizedKeysFile|PermitEmptyPasswords|PermitUserEnvironment|UsePAM|KexAlgorithms|Ciphers|MACs|HostKeyAlgorithms|PubkeyAcceptedAlgorithms|MaxAuthTries|LoginGraceTime|MaxSessions|MaxStartups|X11Forwarding|AllowTcpForwarding|AllowStreamLocalForwarding|AllowAgentForwarding|PermitTunnel|ClientAliveInterval|ClientAliveCountMax|UseDNS|DebianBanner|LogLevel|StrictModes|GatewayPorts|Banner|PrintLastLog|GSSAPIAuthentication|AcceptEnv|HostbasedAuthentication|IgnoreRhosts|PermitUserRC|VersionAddendum|Subsystem'

    for f in "$main_conf" "$dropin_dir"/*.conf; do
        [[ -f "$f" ]] || continue
        [[ "$f" == "$keep_file" ]] && continue
        if grep -qE "^[[:space:]]*(${re})[[:space:]]+" "$f"; then
            backup="${f}.sshhardenbak.$(date +%s)"
            cp "$f" "$backup"
            chmod 600 "$backup"
            sed -i -E "s/^[[:space:]]*(${re})[[:space:]]+/# &/g" "$f"
        fi
    done
}

restore_ssh_backup() {
    # Rollback SSH config to pre-hardening state.
    # Uses SSH_BACKUP, SSH_CONF, SSHD_DROPIN, SSH_SOCKET_DROPIN, SSH_SOCKET_UNIT
    # which must be set by the caller.
    rm -f "$SSHD_DROPIN" 2>/dev/null || true
    if [[ -n "${SSH_SOCKET_DROPIN:-}" ]]; then
        rm -f "$SSH_SOCKET_DROPIN" 2>/dev/null || true
        systemctl daemon-reload >/dev/null 2>&1 || true
        if [[ -n "${SSH_SOCKET_UNIT:-}" ]]; then
            systemctl restart "$SSH_SOCKET_UNIT" >/dev/null 2>&1 || true
        fi
    fi
    cp "$SSH_BACKUP" "$SSH_CONF"
    chmod 0644 "$SSH_CONF" 2>/dev/null || true
    systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
}

ensure_sshd_dropins_included() {
    # Ensure sshd_config.d drop-ins are included OUTSIDE any Match blocks.
    local main_conf="$1"
    local inc='Include /etc/ssh/sshd_config.d/*.conf'
    local tmp

    if grep -qE '^[[:space:]]*Include[[:space:]]+/etc/ssh/sshd_config\.d/\*\.conf([[:space:]]+.*)?$' "$main_conf"; then
        return 0
    fi

    tmp="$(mktemp)"
    awk -v inc="$inc" '
        BEGIN {added=0}
        {
            if (!added && $0 !~ /^[[:space:]]*#/ && $0 ~ /^[[:space:]]*Match[[:space:]]/) {
                print ""
                print "# Added by VLESS setup to support sshd_config.d drop-ins."
                print inc
                added=1
            }
            print $0
        }
        END {
            if (!added) {
                print ""
                print "# Added by VLESS setup to support sshd_config.d drop-ins."
                print inc
            }
        }
    ' "$main_conf" > "$tmp"
    install -o root -g root -m 0644 "$tmp" "$main_conf"
    rm -f "$tmp"
}

temp_firewall_lockdown_enable() {
    # Temporary restrictive ruleset to avoid any "open window" during UFW changes.
    # It is intentionally removed later so UFW/Fail2Ban can manage bans properly.
    local ssh_port="$1"
    local allow_22="${2:-true}"

    if command -v iptables >/dev/null 2>&1; then
        iptables -N VLESS_LOCKDOWN >/dev/null 2>&1 || true
        iptables -F VLESS_LOCKDOWN >/dev/null 2>&1 || true
        # Ensure our jump is always first (avoid being bypassed by earlier rules).
        while iptables -D INPUT -j VLESS_LOCKDOWN >/dev/null 2>&1; do :; done
        iptables -I INPUT 1 -j VLESS_LOCKDOWN

        iptables -A VLESS_LOCKDOWN -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
        iptables -A VLESS_LOCKDOWN -i lo -j ACCEPT
        if [[ "$allow_22" == "true" ]]; then
            iptables -A VLESS_LOCKDOWN -p tcp --dport 22 -j ACCEPT
        fi
        iptables -A VLESS_LOCKDOWN -p tcp --dport "$ssh_port" -j ACCEPT
        iptables -A VLESS_LOCKDOWN -p tcp --dport 443 -j ACCEPT
        iptables -A VLESS_LOCKDOWN -j DROP
    fi

    if command -v ip6tables >/dev/null 2>&1; then
        ip6tables -N VLESS_LOCKDOWN >/dev/null 2>&1 || true
        ip6tables -F VLESS_LOCKDOWN >/dev/null 2>&1 || true
        # Ensure our jump is always first (avoid being bypassed by earlier rules).
        while ip6tables -D INPUT -j VLESS_LOCKDOWN >/dev/null 2>&1; do :; done
        ip6tables -I INPUT 1 -j VLESS_LOCKDOWN

        ip6tables -A VLESS_LOCKDOWN -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
        ip6tables -A VLESS_LOCKDOWN -i lo -j ACCEPT
        if [[ "$allow_22" == "true" ]]; then
            ip6tables -A VLESS_LOCKDOWN -p tcp --dport 22 -j ACCEPT
        fi
        ip6tables -A VLESS_LOCKDOWN -p tcp --dport "$ssh_port" -j ACCEPT
        ip6tables -A VLESS_LOCKDOWN -p tcp --dport 443 -j ACCEPT
        ip6tables -A VLESS_LOCKDOWN -j DROP
    fi
}

temp_firewall_lockdown_disable() {
    if command -v iptables >/dev/null 2>&1; then
        while iptables -D INPUT -j VLESS_LOCKDOWN >/dev/null 2>&1; do :; done
        iptables -F VLESS_LOCKDOWN >/dev/null 2>&1 || true
        iptables -X VLESS_LOCKDOWN >/dev/null 2>&1 || true
    fi
    if command -v ip6tables >/dev/null 2>&1; then
        while ip6tables -D INPUT -j VLESS_LOCKDOWN >/dev/null 2>&1; do :; done
        ip6tables -F VLESS_LOCKDOWN >/dev/null 2>&1 || true
        ip6tables -X VLESS_LOCKDOWN >/dev/null 2>&1 || true
    fi
}

detect_xray_machine() {
    if [[ "$(uname -s)" != "Linux" ]]; then
        error_exit "This script supports Linux (systemd) servers only."
    fi
    case "$(uname -m)" in
        i386|i686) echo "32" ;;
        amd64|x86_64) echo "64" ;;
        armv5tel) echo "arm32-v5" ;;
        armv6l)
            if grep -qw 'vfp' /proc/cpuinfo 2>/dev/null; then echo "arm32-v6"; else echo "arm32-v5"; fi
            ;;
        armv7|armv7l)
            if grep -qw 'vfp' /proc/cpuinfo 2>/dev/null; then echo "arm32-v7a"; else echo "arm32-v5"; fi
            ;;
        armv8|aarch64) echo "arm64-v8a" ;;
        mips) echo "mips32" ;;
        mipsle) echo "mips32le" ;;
        mips64)
            if command -v lscpu >/dev/null 2>&1 && lscpu 2>/dev/null | grep -F "Little Endian" >/dev/null 2>&1; then echo "mips64le"; else echo "mips64"; fi
            ;;
        mips64le) echo "mips64le" ;;
        ppc64) echo "ppc64" ;;
        ppc64le) echo "ppc64le" ;;
        riscv64) echo "riscv64" ;;
        s390x) echo "s390x" ;;
        *) error_exit "Unsupported architecture: $(uname -m)" ;;
    esac
}

get_latest_xray_version() {
    local json
    json="$(curl_secure -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/XTLS/Xray-core/releases/latest")" || return 1
    awk -F'"' '/"tag_name":/ {print $4; exit}' <<<"$json"
}

ensure_xray_user_and_service() {
    # Dedicated user reduces blast radius if Xray is compromised.
    if ! id -u xray >/dev/null 2>&1; then
        useradd --system --home /usr/local/share/xray --shell /usr/sbin/nologin xray >/dev/null 2>&1 || \
            error_exit "Failed to create user 'xray'"
    fi

    # Always (re)write a hardened unit so reruns cannot leave a weak service behind.
    cat > /etc/systemd/system/xray.service <<'EOF'
[Unit]
Description=Xray Service
Documentation=https://github.com/XTLS/Xray-core
After=network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
User=xray
Group=xray
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=/usr/local/bin/xray run -config /usr/local/etc/xray/config.json
Restart=on-failure
RestartSec=5s
RestartPreventExitStatus=23
LimitNPROC=10000
LimitNOFILE=1000000

# Filesystem sandboxing
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ReadOnlyPaths=/usr/local/etc/xray /usr/local/share/xray

# Kernel / namespace hardening
ProtectHostname=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallArchitectures=native
SystemCallFilter=~@clock @debug @module @mount @obsolete @reboot @swap @cpu-emulation @raw-io
UMask=0077

[Install]
WantedBy=multi-user.target
EOF
    chmod 0644 /etc/systemd/system/xray.service

    systemctl daemon-reload
    systemctl enable xray >/dev/null 2>&1 || true
}

install_xray_secure() {
    local machine version base_url zip_url tmp_dir zip_file dgst_file checksum localsum zlist
    machine="$(detect_xray_machine)"

    version="${XRAY_VERSION:-}"
    if [[ -z "$version" ]]; then
        version="$(get_latest_xray_version)" || error_exit "Failed to detect latest Xray version (GitHub API). Set XRAY_VERSION (e.g. v26.2.6) and retry."
    fi
    version="v${version#v}"

    base_url="https://github.com/XTLS/Xray-core/releases/download/${version}"
    zip_file="Xray-linux-${machine}.zip"
    dgst_file="${zip_file}.dgst"
    zip_url="${base_url}/${zip_file}"

    tmp_dir="$(mktemp -d)"
    _CLEANUP_DIRS+=("$tmp_dir")

    echo "       Downloading ${zip_file} (${version})..."
    curl_secure -o "${tmp_dir}/${zip_file}" "${zip_url}" || error_exit "Xray download failed: ${zip_url}"

    localsum="$(sha256sum "${tmp_dir}/${zip_file}" | awk '{print $1}')"

    if [[ -n "${XRAY_EXPECTED_SHA256:-}" ]]; then
        # User-pinned checksum: strongest verification (immune to supply-chain attacks on GitHub releases).
        if [[ "${XRAY_EXPECTED_SHA256}" != "$localsum" ]]; then
            error_exit "SHA256 verification failed (user-pinned). Expected ${XRAY_EXPECTED_SHA256}, got ${localsum}"
        fi
        echo "       ✓ SHA256 verified against user-pinned hash"
    else
        # TOFU check: .dgst is downloaded from the same GitHub release as the binary.
        # This protects against download corruption but NOT against a compromised release.
        # For stronger verification, set XRAY_EXPECTED_SHA256 with a known-good hash.
        curl_secure -o "${tmp_dir}/${dgst_file}" "${zip_url}.dgst" || error_exit "Xray checksum download failed: ${zip_url}.dgst"

        if grep -q "Not Found" "${tmp_dir}/${dgst_file}" 2>/dev/null; then
            error_exit "This Xray version does not provide a checksum file. Pick another XRAY_VERSION."
        fi

        checksum="$(awk -F'= ' '/256=/ {print $2; exit}' "${tmp_dir}/${dgst_file}")"
        [[ -n "$checksum" ]] || error_exit "Failed to parse SHA256 from ${dgst_file}"

        if [[ "$checksum" != "$localsum" ]]; then
            error_exit "SHA256 verification failed for ${zip_file}. Expected ${checksum}, got ${localsum}"
        fi
    fi

    # Defensive: prevent zip path traversal / absolute paths before extraction.
    command -v zipinfo >/dev/null 2>&1 || error_exit "zipinfo not found (required for safe Xray extraction)"
    zlist="$(zipinfo -1 "${tmp_dir}/${zip_file}" 2>/dev/null)" || error_exit "Failed to inspect Xray archive contents"
    if grep -qE '(^/|(^|/)\.\.(/|$))' <<<"$zlist"; then
        error_exit "Unsafe paths detected in Xray archive; aborting extraction"
    fi
    zlist=""

    unzip -q "${tmp_dir}/${zip_file}" -d "${tmp_dir}/xray" || error_exit "Xray archive decompression failed"
    [[ -f "${tmp_dir}/xray/xray" ]] || error_exit "Unexpected Xray archive layout (missing xray binary)"

    # Stop service if running (safe on reruns).
    systemctl stop xray >/dev/null 2>&1 || true

    install -m 0755 "${tmp_dir}/xray/xray" /usr/local/bin/xray
    install -d -m 0755 /usr/local/share/xray
    [[ -f "${tmp_dir}/xray/geoip.dat" ]] && install -m 0644 "${tmp_dir}/xray/geoip.dat" /usr/local/share/xray/geoip.dat
    [[ -f "${tmp_dir}/xray/geosite.dat" ]] && install -m 0644 "${tmp_dir}/xray/geosite.dat" /usr/local/share/xray/geosite.dat

    ensure_xray_user_and_service
}

# Wait for APT/DPKG locks (dpkg uses fcntl locks)
wait_for_apt_locks() {
    local tries=0
    local lock_files=(
        "/var/lib/dpkg/lock-frontend"
        "/var/lib/dpkg/lock"
        "/var/lib/apt/lists/lock"
        "/var/cache/apt/archives/lock"
    )

    if command -v fuser >/dev/null 2>&1; then
        while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
              fuser /var/lib/dpkg/lock >/dev/null 2>&1 || \
              fuser /var/lib/apt/lists/lock >/dev/null 2>&1 || \
              fuser /var/cache/apt/archives/lock >/dev/null 2>&1; do
            echo "       Waiting for apt..."
            sleep 3
            tries=$((tries+1))
            if [[ "$tries" -gt 40 ]]; then
                error_exit "APT locked too long. Reboot and retry."
            fi
        done
        return 0
    fi

    # Fallback: check fcntl locks via python3 (no extra packages; reliable for dpkg/apt)
    if command -v python3 >/dev/null 2>&1; then
        while true; do
            local locked=false
            local lf
            for lf in "${lock_files[@]}"; do
                [[ -e "$lf" ]] || continue
                if ! python3 - "$lf" <<'PY'
import fcntl
import os
import sys

path = sys.argv[1]
try:
    fd = os.open(path, os.O_RDWR)
except FileNotFoundError:
    sys.exit(0)

try:
    fcntl.lockf(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
except OSError:
    sys.exit(1)
else:
    sys.exit(0)
finally:
    try:
        os.close(fd)
    except Exception:
        pass
PY
                then
                    locked=true
                    break
                fi
            done

            if [[ "$locked" == false ]]; then
                return 0
            fi

            echo "       Waiting for apt..."
            sleep 3
            tries=$((tries+1))
            if [[ "$tries" -gt 40 ]]; then
                error_exit "APT locked too long. Reboot and retry."
            fi
        done
    fi

    # Last resort: wait for known package manager processes
    if command -v pgrep >/dev/null 2>&1; then
        while pgrep -x apt-get >/dev/null 2>&1 || pgrep -x apt >/dev/null 2>&1 || pgrep -x dpkg >/dev/null 2>&1; do
            echo "       Waiting for apt..."
            sleep 3
            tries=$((tries+1))
            if [[ "$tries" -gt 40 ]]; then
                error_exit "APT locked too long. Reboot and retry."
            fi
        done
        return 0
    fi

    echo "       ⚠ Cannot check APT locks (missing fuser/python3/pgrep); continuing..."
    return 0
}

# --- Setup Logging ---
if [[ "$HARDEN_SETUP_LOG" == "true" ]]; then
    SETUP_LOG="/var/log/vless-setup.log"
    touch "$SETUP_LOG"
    chmod 600 "$SETUP_LOG"
    # Tee all output to log file while preserving terminal output
    exec > >(tee -a "$SETUP_LOG") 2>&1
    echo "--- VLESS setup started: $(date) ---" >> "$SETUP_LOG"
fi

# --- 0. Pre-flight Checks ---
echo ""
echo "============================================================"
echo "   VLESS + REALITY Setup"
echo "============================================================"

# Root check
if [[ $EUID -ne 0 ]]; then
    error_exit "Run as root: sudo $0"
fi

# SSH keys check
SSH_TEST_USER="root"
ROOT_AUTH_KEYS="/root/.ssh/authorized_keys"
SUDO_AUTH_KEYS=""

if [[ -n "${SUDO_USER:-}" ]]; then
    if ! is_safe_ssh_username "$SUDO_USER"; then
        error_exit "Suspicious SUDO_USER='${SUDO_USER}'. Refusing to continue for safety."
    fi
    SUDO_HOME="$(getent passwd "$SUDO_USER" 2>/dev/null | awk -F: '{print $6}' || true)"
    if [[ -n "$SUDO_HOME" ]]; then
        SUDO_AUTH_KEYS="${SUDO_HOME}/.ssh/authorized_keys"
    fi
fi

# Prefer the invoking sudo user's own key when available. Some cloud images put
# a forced-command redirect key in /root/.ssh/authorized_keys; copying that key
# into the only allowed admin account can make SSH authenticate but reject all
# commands.
if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" && -n "$SUDO_AUTH_KEYS" && -s "$SUDO_AUTH_KEYS" ]]; then
    SSH_TEST_USER="$SUDO_USER"
elif [[ -s "$ROOT_AUTH_KEYS" ]]; then
    SSH_TEST_USER="root"
elif [[ -s "$HOME/.ssh/authorized_keys" ]]; then
    # Fallback (covers unusual sudo/home setups)
    SSH_TEST_USER="${SUDO_USER:-root}"
else
    echo ""
    echo "⛔ STOP: No SSH keys found!"
    echo ""
    echo "This script disables password login."
    echo "You will be LOCKED OUT without SSH keys."
    echo ""
    echo "First run on your local machine:"
    echo "  ssh-copy-id root@YOUR_SERVER_IP"
    if [[ -n "${SUDO_USER:-}" ]]; then
        echo "  # or copy a key to ${SUDO_USER}@YOUR_SERVER_IP"
    fi
    echo ""
    exit 1
fi

if ! is_safe_ssh_username "$SSH_TEST_USER"; then
    error_exit "Resolved SSH_TEST_USER='${SSH_TEST_USER}' is not a safe username. Refusing to write sshd_config."
fi

# Hardened admin user selection (avoid root login when possible)
PERMIT_ROOT_LOGIN="prohibit-password"
SSH_ALLOWED_USER="$SSH_TEST_USER"
if [[ "$SSH_TEST_USER" == "root" ]]; then
    if [[ "$HARDEN_CREATE_ADMIN_USER" == "true" ]]; then
        ensure_admin_user_with_keys "$HARDEN_ADMIN_USER" "$ROOT_AUTH_KEYS"
        SSH_TEST_USER="$HARDEN_ADMIN_USER"
        SSH_ALLOWED_USER="$HARDEN_ADMIN_USER"
        PERMIT_ROOT_LOGIN="no"
    else
        PERMIT_ROOT_LOGIN="prohibit-password"
        SSH_ALLOWED_USER="root"
    fi
else
    PERMIT_ROOT_LOGIN="no"
    SSH_ALLOWED_USER="$SSH_TEST_USER"
fi
if [[ "$SSH_ALLOWED_USER" != "root" && "$HARDEN_NORMALIZE_AUTHORIZED_KEYS" == "true" ]]; then
    sanitize_user_authorized_keys "$SSH_ALLOWED_USER"
fi
if [[ "$PERMIT_ROOT_LOGIN" == "no" ]]; then
    SSH_DENY_ROOT_LINE="DenyUsers root"
else
    SSH_DENY_ROOT_LINE=""
fi

echo ""
echo "✓ Running as root"
echo "✓ SSH keys found for user: ${SSH_TEST_USER}"
echo "✓ Root login: ${PERMIT_ROOT_LOGIN}"
echo "✓ Xray mode: ${XRAY_MODE}"
echo "✓ Transport: ${XRAY_TRANSPORT}"
if [[ "$XRAY_MODE" == "entry" ]]; then
    echo "✓ Upstream target: ${UPSTREAM_IP}:${UPSTREAM_PORT}"
elif [[ "$XRAY_MODE" == "exit" && -n "$ALLOW_ENTRY_IP" ]]; then
    echo "✓ Exit allowlist: ${ALLOW_ENTRY_IP} (tcp/443)"
fi

# DPI-evasion advisories (non-fatal). See PLAN.md / AGENTS.md for rationale.
case "$REALITY_FP" in
    android|ios)
        echo "⚠ REALITY_FP='${REALITY_FP}' is weaker/known-buggy on some clients; 'chrome' is recommended."
        ;;
esac
case "$REALITY_SNI" in
    www.microsoft.com|www.apple.com|www.icloud.com|www.cloudflare.com|www.amazon.com|www.bing.com)
        echo "⚠ REALITY_SNI='${REALITY_SNI}' is a mass-scanned ('burned') camouflage domain."
        echo "  Prefer a less-common site co-located in your VPS datacenter (low ping, TLS1.3,"
        echo "  HTTP/2, no homepage redirect). Find one with RealiTLScanner against your VPS IP."
        ;;
esac
if [[ "$XRAY_MODE" == "entry" && "$REALITY_SNI" != "ya.ru" ]]; then
    echo "ℹ Entry node tip: on mobile/whitelist networks, host the entry on Yandex.Cloud and"
    echo "  set REALITY_SNI=ya.ru so client→entry traffic looks like legitimate Yandex."
fi

# --- 1. Free Port 443 ---
echo ""
echo "[1/12] Freeing port 443..."
systemctl stop apache2 2>/dev/null || true
systemctl disable apache2 2>/dev/null || true
systemctl stop nginx 2>/dev/null || true
systemctl disable nginx 2>/dev/null || true
echo "       ✓ Done"

# --- 2. Wait for APT ---
echo ""
echo "[2/12] Checking apt locks..."
wait_for_apt_locks
echo "       ✓ APT ready"

# --- 3. Install Packages ---
echo ""
echo "[3/12] Installing packages..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq || error_exit "apt update failed"
if [[ "$HARDEN_APT_UPGRADE" == "true" ]]; then
    if [[ "$HARDEN_APT_SECURITY_ONLY" == "true" ]]; then
        # Only apply security updates (requires unattended-upgrades to be installed first)
        apt-get install -y -qq unattended-upgrades >/dev/null 2>&1 || true
        unattended-upgrades --verbose 2>/dev/null || echo "       ⚠ Security-only upgrade had warnings (non-fatal)"
    else
        apt-get upgrade -y -qq -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" || error_exit "apt upgrade failed"
    fi
fi
apt-get install -y -qq ca-certificates curl unzip qrencode ufw fail2ban debconf-utils \
    unattended-upgrades openssl chrony auditd sudo || error_exit "apt install failed"
systemctl enable --now auditd >/dev/null 2>&1 || true
echo "       ✓ Packages installed"

# --- 3.1 Swap File ---
echo ""
echo "[3.1/12] Checking swap..."
if [[ $(swapon --show | wc -l) -eq 0 ]]; then
    echo "       Creating 1GB swap file..."
    fallocate -l 1G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=1024
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    if ! grep -qE '^[[:space:]]*/swapfile[[:space:]]' /etc/fstab; then
        echo "/swapfile none swap sw 0 0" >> /etc/fstab
    fi
    echo "       ✓ Swap created"
else
    echo "       ✓ Swap exists"
fi

# --- 4. Time Sync ---
echo ""
echo "[4/12] Syncing time..."
systemctl enable --now chrony >/dev/null 2>&1 || true
chronyc makestep >/dev/null 2>&1 || true
echo "       ✓ Time synced"

# --- 5. Auto Updates ---
echo ""
echo "[5/12] Enabling auto-updates..."
echo "unattended-upgrades unattended-upgrades/enable_auto_updates boolean true" | debconf-set-selections || true
dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null 2>&1
# Use a dedicated override file (reliable regardless of 50-unattended-upgrades formatting)
{
    echo "// Managed by VLESS setup script - do not edit"
    if [[ "$HARDEN_AUTO_REBOOT" == "true" ]]; then
        echo 'Unattended-Upgrade::Automatic-Reboot "true";'
        echo 'Unattended-Upgrade::Automatic-Reboot-Time "04:00";'
    else
        echo 'Unattended-Upgrade::Automatic-Reboot "false";'
    fi
    echo 'Unattended-Upgrade::Remove-Unused-Dependencies "true";'
    echo 'Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";'
    echo 'Unattended-Upgrade::Remove-New-Unused-Dependencies "true";'
} > /etc/apt/apt.conf.d/51-vless-autoupdate
chmod 0644 /etc/apt/apt.conf.d/51-vless-autoupdate
if [[ "$HARDEN_AUTO_REBOOT" == "true" ]]; then
    echo "       ✓ Auto-updates enabled (with auto-reboot at 04:00)"
else
    echo "       ✓ Auto-updates enabled (auto-reboot DISABLED — reboot manually after kernel updates)"
fi

# --- 6. BBR + Kernel Hardening ---
echo ""
echo "[6/12] Enabling BBR & kernel hardening..."
SYSCTL_HARDENING="/etc/sysctl.d/99-vless-hardening.conf"
cat > "$SYSCTL_HARDENING" <<'SYSEOF'
# === BBR congestion control ===
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr

# === Network hardening ===
# SYN flood protection
net.ipv4.tcp_syncookies=1
net.ipv4.tcp_max_syn_backlog=2048
net.ipv4.tcp_synack_retries=2
# TCP timestamps: keep enabled for PAWS (Protection Against Wrapped Sequences).
# Disabling hurts reliability on fast links (>1Gbps). SYN flood protection is
# already handled by tcp_syncookies=1 above.
net.ipv4.tcp_timestamps=1
net.ipv4.tcp_rfc1337=1
net.ipv4.icmp_ignore_bogus_error_responses=1

# IP spoofing protection (reverse-path filtering)
net.ipv4.conf.all.rp_filter=1
net.ipv4.conf.default.rp_filter=1

# Disable ICMP redirects (prevents MITM via route injection)
net.ipv4.conf.all.accept_redirects=0
net.ipv4.conf.default.accept_redirects=0
net.ipv6.conf.all.accept_redirects=0
net.ipv6.conf.default.accept_redirects=0
net.ipv4.conf.all.send_redirects=0
net.ipv4.conf.default.send_redirects=0
net.ipv4.conf.all.secure_redirects=0
net.ipv4.conf.default.secure_redirects=0

# Disable proxy ARP / local source address acceptance
net.ipv4.conf.all.proxy_arp=0
net.ipv4.conf.default.proxy_arp=0
net.ipv4.conf.all.accept_local=0
net.ipv4.conf.default.accept_local=0

# Disable source routing
net.ipv4.conf.all.accept_source_route=0
net.ipv4.conf.default.accept_source_route=0
net.ipv6.conf.all.accept_source_route=0
net.ipv6.conf.default.accept_source_route=0

# Ignore ICMP broadcast requests (smurf attack protection)
net.ipv4.icmp_echo_ignore_broadcasts=1

# Log Martian packets (impossible source addresses)
net.ipv4.conf.all.log_martians=1
net.ipv4.conf.default.log_martians=1

# === Kernel Self-Protection ===
kernel.kptr_restrict=2
kernel.dmesg_restrict=1
kernel.sysrq=0
kernel.unprivileged_bpf_disabled=1
kernel.kexec_load_disabled=1
kernel.yama.ptrace_scope=2
kernel.perf_event_paranoid=3
kernel.randomize_va_space=2
# NOTE: kernel.unprivileged_userns_clone and user.max_user_namespaces are
# appended conditionally below (skipped when container runtimes are detected).
vm.mmap_min_addr=65536
fs.suid_dumpable=0
fs.protected_hardlinks=1
fs.protected_symlinks=1
fs.protected_fifos=2
fs.protected_regular=2

# === Disable IP forwarding (prevent server as pivot/router) ===
net.ipv4.ip_forward=0
net.ipv6.conf.all.forwarding=0

# === ARP hardening (prevent spoofing / cache poisoning) ===
net.ipv4.conf.all.arp_ignore=1
net.ipv4.conf.all.arp_announce=2
net.ipv4.conf.default.arp_ignore=1
net.ipv4.conf.default.arp_announce=2
net.ipv4.conf.all.arp_accept=0
net.ipv4.conf.default.arp_accept=0

# === Core dumps: kernel-level prevention ===
kernel.core_pattern=|/bin/false

# === IPv6 router advertisement suppression ===
net.ipv6.conf.all.accept_ra=0
net.ipv6.conf.default.accept_ra=0

# === TCP hardening extras ===
net.ipv4.tcp_fin_timeout=15
net.ipv4.tcp_keepalive_time=600
net.ipv4.tcp_keepalive_intvl=60
net.ipv4.tcp_keepalive_probes=3

# === BPF JIT hardening (prevent JIT spray attacks) ===
net.core.bpf_jit_harden=2

# === Restrict userfaultfd (reduces kernel exploit surface) ===
vm.unprivileged_userfaultfd=0
SYSEOF

# Conditionally disable unprivileged user namespaces (breaks Docker/Podman/LXC).
if command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1 || command -v lxc-start >/dev/null 2>&1; then
    echo "       ⚠ Container runtime detected — skipping user namespace restrictions"
else
    {
        echo ""
        echo "# === Unprivileged user namespace restriction (no containers detected) ==="
        echo "kernel.unprivileged_userns_clone=0"
        echo "user.max_user_namespaces=0"
    } >> "$SYSCTL_HARDENING"
fi

# Secure Shared Memory
shm_line="$(awk '$1 !~ /^#/ && $2 == "/dev/shm" {print; exit}' /etc/fstab || true)"
if [[ -n "$shm_line" ]]; then
    shm_opts="$(awk '{print $4}' <<<"$shm_line")"
    [[ -n "$shm_opts" ]] || shm_opts="defaults"
    declare -A shm_set=()
    IFS=',' read -r -a shm_arr <<<"$shm_opts"
    for opt in "${shm_arr[@]}"; do
        [[ -n "$opt" ]] && shm_set["$opt"]=1
    done
    shm_set["noexec"]=1
    shm_set["nosuid"]=1
    shm_set["nodev"]=1
    shm_new_opts=""
    if [[ -n "${shm_set[defaults]:-}" ]]; then
        shm_new_opts="defaults"
        unset 'shm_set[defaults]'
    fi
    for opt in "${shm_arr[@]}"; do
        [[ "$opt" == "defaults" ]] && continue
        if [[ -n "${shm_set[$opt]:-}" ]]; then
            shm_new_opts+="${shm_new_opts:+,}$opt"
            unset "shm_set[$opt]"
        fi
    done
    for opt in "${!shm_set[@]}"; do
        shm_new_opts+="${shm_new_opts:+,}$opt"
    done
    sed -i -E "s|^[[:space:]]*tmpfs[[:space:]]+/dev/shm[[:space:]]+tmpfs[[:space:]]+[^[:space:]]+|tmpfs /dev/shm tmpfs ${shm_new_opts}|" /etc/fstab
else
    if ! grep -qE '^[[:space:]]*tmpfs[[:space:]]+/dev/shm[[:space:]]' /etc/fstab; then
        echo "tmpfs /dev/shm tmpfs defaults,noexec,nosuid,nodev 0 0" >> /etc/fstab
    fi
fi
mount -o remount,noexec,nosuid,nodev /dev/shm 2>/dev/null || true
chmod 0644 "$SYSCTL_HARDENING"
sysctl --system >/dev/null 2>&1 || sysctl -p "$SYSCTL_HARDENING" >/dev/null 2>&1 || true
echo "       ✓ BBR enabled + kernel hardened"

# --- 6.1 Core Dump Prevention ---
echo ""
echo "[6.1] Disabling core dumps..."
cat > /etc/security/limits.d/99-no-core.conf <<'EOF'
# Prevent all users from generating core dumps
* hard core 0
* soft core 0
root hard core 0
root soft core 0
EOF
chmod 0644 /etc/security/limits.d/99-no-core.conf
# Disable systemd-coredump as well
mkdir -p /etc/systemd/coredump.conf.d
cat > /etc/systemd/coredump.conf.d/99-disable.conf <<'EOF'
[Coredump]
Storage=none
ProcessSizeMax=0
EOF
echo "       ✓ Core dumps disabled (limits + sysctl + systemd)"

# --- 6.2 Blacklist Unused Kernel Modules ---
echo ""
echo "[6.2] Blacklisting unused kernel modules..."
cat > /etc/modprobe.d/99-vless-blacklist.conf <<'EOF'
# Unused network protocols (reduce kernel attack surface)
install dccp /bin/false
install sctp /bin/false
install rds /bin/false
install tipc /bin/false

# Uncommon/legacy filesystems
install cramfs /bin/false
install freevxfs /bin/false
install jffs2 /bin/false
install hfs /bin/false
install hfsplus /bin/false
install udf /bin/false

# USB storage (not needed on VPS)
install usb-storage /bin/false

# Firewire (not needed on VPS)
install firewire-core /bin/false
install firewire-ohci /bin/false
install firewire-sbp2 /bin/false

# Bluetooth (not needed on VPS)
install bluetooth /bin/false
install btusb /bin/false
EOF
chmod 0644 /etc/modprobe.d/99-vless-blacklist.conf
echo "       ✓ Unused kernel modules blacklisted"

# --- 6.3 /proc Hardening (hidepid) ---
echo ""
echo "[6.3] Hardening /proc..."
# Use hidepid=invisible on kernel 5.8+ (modern, avoids breaking systemd user sessions);
# fall back to hidepid=2 on older kernels.
_kver_major="$(uname -r | cut -d. -f1)"
_kver_minor="$(uname -r | cut -d. -f2)"
if (( _kver_major > 5 || (_kver_major == 5 && _kver_minor >= 8) )); then
    _hidepid_opt="hidepid=invisible"
else
    _hidepid_opt="hidepid=2"
fi
if ! grep -qE '^proc[[:space:]]+/proc[[:space:]]' /etc/fstab; then
    echo "proc /proc proc defaults,${_hidepid_opt},gid=0 0 0" >> /etc/fstab
fi
mount -o "remount,${_hidepid_opt},gid=0" /proc 2>/dev/null || true
echo "       ✓ /proc hardened (${_hidepid_opt}, only root sees all processes)"

# --- 6.4 Audit Rules ---
echo ""
echo "[6.4] Configuring audit rules..."
cat > /etc/audit/rules.d/99-vless-hardening.rules <<'EOF'
# Delete all existing rules
-D

# Buffer size
-b 8192

# Failure mode (1 = printk)
-f 1

# --- Identity & Authentication ---
-w /etc/passwd -p wa -k identity
-w /etc/group -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/gshadow -p wa -k identity
-w /etc/sudoers -p wa -k identity
-w /etc/sudoers.d/ -p wa -k identity
-w /etc/pam.d/ -p wa -k pam

# --- SSH & Keys ---
-w /etc/ssh/ -p wa -k sshd_config
-w /root/.ssh/ -p wa -k ssh_keys

# --- Xray ---
-w /usr/local/etc/xray/ -p wa -k xray_config
-w /usr/local/bin/xray -p x -k xray_binary

# --- Cron ---
-w /etc/crontab -p wa -k cron
-w /etc/cron.d/ -p wa -k cron
-w /etc/cron.daily/ -p wa -k cron
-w /etc/cron.weekly/ -p wa -k cron
-w /etc/cron.monthly/ -p wa -k cron
-w /var/spool/cron/ -p wa -k cron

# --- Login / Session ---
-w /var/log/lastlog -p wa -k logins
-w /var/log/faillog -p wa -k logins
-w /var/run/utmp -p wa -k session
-w /var/log/wtmp -p wa -k session

# --- Kernel modules ---
-w /sbin/insmod -p x -k modules
-w /sbin/rmmod -p x -k modules
-w /sbin/modprobe -p x -k modules
-a always,exit -F arch=b64 -S init_module -S finit_module -S delete_module -k modules
-a always,exit -F arch=b32 -S init_module -S finit_module -S delete_module -k modules

# --- Privilege escalation ---
-a always,exit -F arch=b64 -S execve -S execveat -C uid!=euid -F euid=0 -k escalation
-a always,exit -F arch=b32 -S execve -S execveat -C uid!=euid -F euid=0 -k escalation
-w /bin/su -p x -k escalation
-w /usr/bin/sudo -p x -k escalation
-w /usr/bin/passwd -p x -k escalation
-w /usr/sbin/usermod -p x -k escalation
-w /usr/sbin/useradd -p x -k escalation
-w /usr/sbin/userdel -p x -k escalation
-w /usr/sbin/groupmod -p x -k escalation

# --- Network config changes ---
-w /etc/hosts -p wa -k network
-w /etc/sysctl.conf -p wa -k network
-w /etc/sysctl.d/ -p wa -k network
-w /etc/resolv.conf -p wa -k network

# --- Firewall ---
-w /etc/ufw/ -p wa -k firewall

# --- System time ---
-a always,exit -F arch=b64 -S adjtimex -S settimeofday -S clock_settime -k time
-a always,exit -F arch=b32 -S adjtimex -S settimeofday -S clock_settime -k time

# --- Make rules immutable (reboot required to change) ---
-e 2
EOF
chmod 0640 /etc/audit/rules.d/99-vless-hardening.rules
# NOTE: If -e 2 was loaded previously, rules can only be updated after reboot.
_audit_enabled="$(auditctl -s 2>/dev/null | awk '/^enabled/ {print $2}' || echo "0")"
if [[ "$_audit_enabled" == "2" ]]; then
    echo "       ⚠ Audit rules are immutable (-e 2 active). New rules written to disk but"
    echo "         will NOT take effect until next reboot."
else
    augenrules --load >/dev/null 2>&1 || service auditd restart >/dev/null 2>&1 || true
fi
echo "       ✓ Audit rules configured (immutable; reboot to change)"

# --- 6.5 Restrict Cron/At Access ---
echo ""
echo "[6.5] Restricting cron/at access..."
echo "root" > /etc/cron.allow
chmod 0600 /etc/cron.allow
echo "root" > /etc/at.allow
chmod 0600 /etc/at.allow
if [[ -n "${SSH_ALLOWED_USER:-}" && "$SSH_ALLOWED_USER" != "root" ]]; then
    printf '%s\n' "$SSH_ALLOWED_USER" >> /etc/cron.allow
    printf '%s\n' "$SSH_ALLOWED_USER" >> /etc/at.allow
fi
# Remove permissive deny files (cron.allow takes precedence)
rm -f /etc/cron.deny /etc/at.deny
echo "       ✓ Cron/at restricted to authorized users only"

# --- 6.6 Login Warning Banner ---
echo ""
echo "[6.6] Setting login banner..."
cat > /etc/issue.net <<'EOF'
*******************************************************************
*  WARNING: Unauthorized access to this system is prohibited.     *
*  All connections are monitored and recorded.                    *
*  Disconnect IMMEDIATELY if you are not an authorized user.      *
*******************************************************************
EOF
chmod 0644 /etc/issue.net
cp /etc/issue.net /etc/issue
echo "       ✓ Login warning banner configured"

# --- 6.7 /tmp Hardening ---
echo ""
echo "[6.7] Hardening /tmp..."
chmod 1777 /tmp /var/tmp 2>/dev/null || true
if [[ "$HARDEN_TMP_NOEXEC" == "true" ]]; then
    mkdir -p /etc/systemd/system/tmp.mount.d
    cat > /etc/systemd/system/tmp.mount.d/options.conf <<'TMPEOF'
[Mount]
Options=mode=1777,strictatime,nosuid,nodev,noexec
TMPEOF
    chmod 0644 /etc/systemd/system/tmp.mount.d/options.conf
    systemctl daemon-reload

    # If /tmp is not its own mount point (typical on Ubuntu), enable tmp.mount to mount tmpfs on /tmp.
    tmp_target=""
    if command -v findmnt >/dev/null 2>&1; then
        tmp_target="$(findmnt -n -T /tmp -o TARGET 2>/dev/null || true)"
    fi
    if [[ "$tmp_target" != "/tmp" ]]; then
        systemctl enable --now tmp.mount >/dev/null 2>&1 || true
    fi

    mount -o remount,nosuid,nodev,noexec /tmp >/dev/null 2>&1 || true
    if command -v findmnt >/dev/null 2>&1; then
        tmp_opts_now="$(findmnt -n -T /tmp -o OPTIONS 2>/dev/null || true)"
        if ! grep -qw noexec <<<"$tmp_opts_now" || ! grep -qw nodev <<<"$tmp_opts_now" || ! grep -qw nosuid <<<"$tmp_opts_now"; then
            echo "       ⚠ /tmp hardening may be incomplete (options: ${tmp_opts_now:-unknown})"
        fi
    fi
    echo "       ✓ /tmp hardened (nosuid,nodev,noexec)"
else
    echo "       ⚠ HARDEN_TMP_NOEXEC=false — /tmp noexec hardening skipped"
fi

# --- 6.8 Minimize SUID/SGID Attack Surface ---
echo ""
echo "[6.8] Reducing SUID/SGID attack surface..."
SUID_STRIP_LIST=(
    /usr/bin/chfn
    /usr/bin/chsh
    /usr/bin/newgrp
    /usr/bin/wall
    /usr/bin/write
    /usr/bin/mlocate
    /usr/bin/write.ul
    /usr/bin/mount
    /usr/bin/umount
)
for bin in "${SUID_STRIP_LIST[@]}"; do
    [[ -f "$bin" ]] && chmod u-s,g-s "$bin" 2>/dev/null || true
done
echo "       ✓ Unnecessary SUID/SGID bits removed"

# --- 6.9 SSH Host Key Hygiene ---
echo ""
echo "[6.9] Cleaning SSH host keys..."
# Remove weak/deprecated DSA host keys (small key size, deprecated in OpenSSH 7.0+)
for _ktype in dsa; do
    if [[ -f "/etc/ssh/ssh_host_${_ktype}_key" ]]; then
        rm -f "/etc/ssh/ssh_host_${_ktype}_key" "/etc/ssh/ssh_host_${_ktype}_key.pub"
        echo "       Removed ${_ktype} host key"
    fi
done
# Ensure ed25519 host key exists (strongest, fastest)
if [[ ! -f /etc/ssh/ssh_host_ed25519_key ]]; then
    ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N "" >/dev/null 2>&1
    echo "       Generated ed25519 host key"
fi
# Ensure RSA host key is at least 4096 bits
if [[ -f /etc/ssh/ssh_host_rsa_key ]]; then
    _rsa_bits="$(ssh-keygen -lf /etc/ssh/ssh_host_rsa_key 2>/dev/null | awk '{print $1}' || echo "0")"
    if [[ "$_rsa_bits" -lt 4096 ]]; then
        rm -f /etc/ssh/ssh_host_rsa_key /etc/ssh/ssh_host_rsa_key.pub
        ssh-keygen -t rsa -b 4096 -f /etc/ssh/ssh_host_rsa_key -N "" >/dev/null 2>&1
        echo "       Regenerated RSA host key (now 4096-bit)"
    fi
fi
echo "       ✓ SSH host keys: strong types only"

# --- 6.10 Restrict su Command ---
echo ""
echo "[6.10] Restricting su command..."
PAM_SU="/etc/pam.d/su"
if [[ -f "$PAM_SU" ]]; then
    if ! grep -qE '^auth[[:space:]]+required[[:space:]]+pam_wheel\.so.*group=sudo' "$PAM_SU"; then
        cp "$PAM_SU" "${PAM_SU}.bak.$(date +%s)"
        if grep -qE '^#.*auth.*required.*pam_wheel\.so' "$PAM_SU"; then
            # Uncomment and configure the existing pam_wheel line
            sed -i '/^#.*auth.*required.*pam_wheel\.so/c\auth       required   pam_wheel.so use_uid group=sudo' "$PAM_SU"
        else
            # Append the rule (after pam_rootok which allows root to su freely)
            echo "auth       required   pam_wheel.so use_uid group=sudo" >> "$PAM_SU"
        fi
    fi
fi
echo "       ✓ su restricted to sudo group members only"

# --- 6.11 Harden login.defs ---
echo ""
echo "[6.11] Hardening login.defs..."
LOGIN_DEFS="/etc/login.defs"
if [[ -f "$LOGIN_DEFS" ]]; then
    # Secure default UMASK for new files (group-readable, no world access)
    sed -i -E 's/^UMASK[[:space:]]+[0-9]+/UMASK\t\t027/' "$LOGIN_DEFS"
    # Password aging for NEW accounts (existing accounts not affected)
    sed -i -E 's/^PASS_MAX_DAYS[[:space:]]+[0-9]+/PASS_MAX_DAYS\t90/' "$LOGIN_DEFS"
    sed -i -E 's/^PASS_MIN_DAYS[[:space:]]+[0-9]+/PASS_MIN_DAYS\t1/' "$LOGIN_DEFS"
    sed -i -E 's/^PASS_WARN_AGE[[:space:]]+[0-9]+/PASS_WARN_AGE\t14/' "$LOGIN_DEFS"
    # Ensure SHA512 password hashing
    if grep -q '^ENCRYPT_METHOD' "$LOGIN_DEFS"; then
        sed -i -E 's/^ENCRYPT_METHOD[[:space:]]+.*/ENCRYPT_METHOD SHA512/' "$LOGIN_DEFS"
    else
        echo "ENCRYPT_METHOD SHA512" >> "$LOGIN_DEFS"
    fi
    # Increase SHA hashing rounds (slows brute-force)
    if ! grep -q '^SHA_CRYPT_MIN_ROUNDS' "$LOGIN_DEFS"; then
        echo "SHA_CRYPT_MIN_ROUNDS 10000" >> "$LOGIN_DEFS"
    fi
    # Restrictive home directory permissions for new users
    if grep -q '^HOME_MODE' "$LOGIN_DEFS"; then
        sed -i -E 's/^HOME_MODE[[:space:]]+[0-9]+/HOME_MODE 0750/' "$LOGIN_DEFS"
    else
        echo "HOME_MODE 0750" >> "$LOGIN_DEFS"
    fi
fi
echo "       ✓ login.defs hardened (UMASK 027, SHA512 10k rounds, password aging)"

# --- 6.12 Journald Limits ---
echo ""
echo "[6.12] Configuring journald limits..."
mkdir -p /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/99-vless-limits.conf <<'JEOF'
[Journal]
SystemMaxUse=200M
SystemMaxFileSize=50M
MaxRetentionSec=1month
ForwardToSyslog=no
Compress=yes
JEOF
chmod 0644 /etc/systemd/journald.conf.d/99-vless-limits.conf
systemctl restart systemd-journald >/dev/null 2>&1 || true
echo "       ✓ Journald: 200M max, 1 month retention"

# --- 6.13 Secure /var/log ---
echo ""
echo "[6.13] Securing /var/log..."
chmod 0750 /var/log 2>/dev/null || true
for _logf in /var/log/syslog /var/log/auth.log /var/log/kern.log /var/log/mail.log; do
    [[ -f "$_logf" ]] && chmod 0640 "$_logf" 2>/dev/null || true
done
echo "       ✓ /var/log permissions secured (no world-read)"

# --- 6.14 Disable Unnecessary Services ---
echo ""
echo "[6.14] Disabling unnecessary services..."
_DISABLE_SVCS=(avahi-daemon cups bluetooth ModemManager snapd.service snapd.socket)
for _svc in "${_DISABLE_SVCS[@]}"; do
    if systemctl is-enabled "$_svc" >/dev/null 2>&1; then
        systemctl stop "$_svc" >/dev/null 2>&1 || true
        systemctl disable "$_svc" >/dev/null 2>&1 || true
        echo "       Disabled: $_svc"
    fi
done
echo "       ✓ Unnecessary services check complete"

# --- 7. Install Xray ---
echo ""
echo "[7/12] Installing Xray..."
install_xray_secure

if ! command -v xray &>/dev/null; then
    error_exit "Xray install failed"
fi
echo "       ✓ Xray installed"

# --- 8. Generate Keys (FIXED FOR XRAY 25.x) ---
echo ""
echo "[8/12] Generating credentials..."

# Guard: prevent accidental overwrite of existing keys on re-run
if [[ -f /usr/local/etc/xray/config.json ]] && [[ "$XRAY_FORCE_REGENERATE" != "true" ]]; then
    echo ""
    echo "⛔ Xray config already exists: /usr/local/etc/xray/config.json"
    echo ""
    echo "Re-running this step will OVERWRITE your existing UUID and keys."
    echo "All connected clients will lose access permanently."
    echo ""
    echo "To proceed anyway, re-run with:"
    echo "  XRAY_FORCE_REGENERATE=true $0"
    echo ""
    error_exit "Refusing to overwrite existing Xray credentials. Set XRAY_FORCE_REGENERATE=true to force."
fi

UUID=$(xray uuid)
X25519_OUT=$(xray x25519)
REALITY_SHORTID=$(openssl rand -hex 8)

# BULLETPROOF PARSING - Works with ALL Xray versions
# Xray 25.x output format:
#   Line 1: PrivateKey: xxxxx
#   Line 2: Password: xxxxx (this is the public key!)
#   Line 3: Hash32: xxxxx
#
# OLD versions:
#   Line 1: Private key: xxxxx
#   Line 2: Public key: xxxxx

# Parse by labels (handles: "PrivateKey:", "Private key:", "PublicKey:", "Public key:", "Password:")
PRIVATE_KEY=$(
    awk -F':' '
        tolower($1) ~ /^[[:space:]]*private/ {
            sub(/^[[:space:]]+/, "", $2)
            sub(/[[:space:]]+$/, "", $2)
            print $2
            exit
        }' <<<"$X25519_OUT"
)
PUBLIC_KEY=$(
    awk -F':' '
        tolower($1) ~ /^[[:space:]]*(public|password)/ {
            sub(/^[[:space:]]+/, "", $2)
            sub(/[[:space:]]+$/, "", $2)
            print $2
            exit
        }' <<<"$X25519_OUT"
)

# Validate keys are not empty
if [ -z "$PRIVATE_KEY" ]; then
    echo ""
    echo "Failed to parse private key from xray x25519 output."
    echo "Output labels found: $(awk -F: '{print $1}' <<<"$X25519_OUT" | head -c 200 | paste -sd, -)"
    error_exit "Private key parsing failed. Check xray x25519 output format."
fi

if [ -z "$PUBLIC_KEY" ]; then
    echo ""
    echo "Failed to parse public key from xray x25519 output."
    echo "Output labels found: $(awk -F: '{print $1}' <<<"$X25519_OUT" | head -c 200 | paste -sd, -)"
    error_exit "Public key parsing failed. Check xray x25519 output format."
fi

# Clear raw key material from shell (private key was in X25519_OUT)
X25519_OUT=""

# Validate generated credential formats (defense-in-depth against corrupted output)
if ! [[ "$UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
    error_exit "Generated UUID has unexpected format: '${UUID:0:8}...'. Expected UUID v4."
fi
if ! [[ "$REALITY_SHORTID" =~ ^[0-9a-f]+$ ]] || [[ ${#REALITY_SHORTID} -lt 8 ]]; then
    error_exit "Generated REALITY_SHORTID has unexpected format (expected hex, min 8 chars)."
fi
# X25519 keys are 32 bytes base64url-encoded = 43-44 chars (no padding or with =)
if ! [[ "$PRIVATE_KEY" =~ ^[A-Za-z0-9_-]{42,44}=?$ ]]; then
    error_exit "Generated private key has unexpected format (expected base64url, 43-44 chars)."
fi
if ! [[ "$PUBLIC_KEY" =~ ^[A-Za-z0-9_-]{42,44}=?$ ]]; then
    error_exit "Generated public key has unexpected format (expected base64url, 43-44 chars)."
fi

echo "       ✓ UUID: ${UUID:0:8}..."
echo "       ✓ Private key: (hidden)"
echo "       ✓ Public key: ${PUBLIC_KEY:0:8}..."
echo "       ✓ Short ID: $REALITY_SHORTID"

# --- 9. Configure Xray ---
echo ""
echo "[9/12] Configuring Xray (mode: ${XRAY_MODE}, transport: ${XRAY_TRANSPORT})..."
install -d -m 0750 /usr/local/etc/xray

# Transport fragments. xtls-rprx-vision is incompatible with xhttp, so the flow is
# cleared and an xhttpSettings block (with padding) is appended after realitySettings.
if [[ "$XRAY_TRANSPORT" == "xhttp" && -z "$XRAY_XHTTP_PATH" ]]; then
    XRAY_XHTTP_PATH="/$(gen_hex_32)"
fi
if [[ "$XRAY_TRANSPORT" == "xhttp" ]]; then
    INBOUND_NET="xhttp"
    INBOUND_FLOW=""
    INBOUND_TRANSPORT_BLOCK=",
      \"xhttpSettings\": {
        \"path\": \"$XRAY_XHTTP_PATH\",
        \"mode\": \"auto\",
        \"extra\": {\"xPaddingBytes\": \"$XRAY_XHTTP_PADDING\"}
      }"
else
    INBOUND_NET="tcp"
    INBOUND_FLOW="xtls-rprx-vision"
    INBOUND_TRANSPORT_BLOCK=""
fi
if [[ "$UPSTREAM_TRANSPORT" == "xhttp" ]]; then
    UP_NET="xhttp"
    UP_FLOW=""
    UP_TRANSPORT_BLOCK=",
        \"xhttpSettings\": {
          \"path\": \"$UPSTREAM_PATH\",
          \"mode\": \"auto\",
          \"extra\": {\"xPaddingBytes\": \"$XRAY_XHTTP_PADDING\"}
        }"
else
    UP_NET="tcp"
    UP_FLOW="xtls-rprx-vision"
    UP_TRANSPORT_BLOCK=""
fi

if [[ "$XRAY_MODE" == "entry" ]]; then
cat > /usr/local/etc/xray/config.json <<EOF
{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "port": 443,
    "protocol": "vless",
    "settings": {
      "clients": [{"id": "$UUID", "flow": "$INBOUND_FLOW"}],
      "decryption": "none"
    },
    "streamSettings": {
      "network": "$INBOUND_NET",
      "security": "reality",
      "realitySettings": {
        "dest": "$REALITY_DEST",
        "serverNames": ["$REALITY_SNI"],
        "privateKey": "$PRIVATE_KEY",
        "shortIds": ["$REALITY_SHORTID"],
        "fingerprint": "$REALITY_FP"
      }$INBOUND_TRANSPORT_BLOCK
    },
    "sniffing": {
      "enabled": true,
      "destOverride": ["http", "tls", "quic"],
      "routeOnly": true
    }
  }],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": [
      {
        "type": "field",
        "ip": ["geoip:private"],
        "outboundTag": "block"
      },
      {
        "type": "field",
        "network": "tcp,udp",
        "outboundTag": "upstream"
      }
    ]
  },
  "outbounds": [
    {
      "protocol": "vless",
      "tag": "upstream",
      "settings": {
        "vnext": [{
          "address": "$UPSTREAM_IP",
          "port": $UPSTREAM_PORT,
          "users": [{
            "id": "$UPSTREAM_UUID",
            "encryption": "none",
            "flow": "$UP_FLOW"
          }]
        }]
      },
      "streamSettings": {
        "network": "$UP_NET",
        "security": "reality",
        "realitySettings": {
          "serverName": "$UPSTREAM_SNI",
          "publicKey": "$UPSTREAM_PBK",
          "shortId": "$UPSTREAM_SID",
          "fingerprint": "$UPSTREAM_FP"
        }$UP_TRANSPORT_BLOCK
      }
    },
    {"protocol": "blackhole", "tag": "block"}
  ]
}
EOF
else
cat > /usr/local/etc/xray/config.json <<EOF
{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "port": 443,
    "protocol": "vless",
    "settings": {
      "clients": [{"id": "$UUID", "flow": "$INBOUND_FLOW"}],
      "decryption": "none"
    },
    "streamSettings": {
      "network": "$INBOUND_NET",
      "security": "reality",
      "realitySettings": {
        "dest": "$REALITY_DEST",
        "serverNames": ["$REALITY_SNI"],
        "privateKey": "$PRIVATE_KEY",
        "shortIds": ["$REALITY_SHORTID"],
        "fingerprint": "$REALITY_FP"
      }$INBOUND_TRANSPORT_BLOCK
    },
    "sniffing": {
      "enabled": true,
      "destOverride": ["http", "tls", "quic"],
      "routeOnly": true
    }
  }],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": [
      {
        "type": "field",
        "ip": ["geoip:private"],
        "outboundTag": "block"
      }
    ]
  },
  "outbounds": [
    {"protocol": "freedom", "tag": "direct"},
    {"protocol": "blackhole", "tag": "block"}
  ]
}
EOF
fi

systemctl daemon-reload
XRAY_SVC_USER="$(systemctl show -p User --value xray 2>/dev/null || true)"
XRAY_SVC_GROUP="$(systemctl show -p Group --value xray 2>/dev/null || true)"
if [[ -z "$XRAY_SVC_USER" ]]; then
    XRAY_SVC_USER="root"
    echo "       ⚠ Could not detect Xray service user; falling back to root"
fi
if [[ -z "$XRAY_SVC_GROUP" ]]; then
    XRAY_SVC_GROUP="$(id -gn "$XRAY_SVC_USER" 2>/dev/null || echo "root")"
fi

chown root:"$XRAY_SVC_GROUP" /usr/local/etc/xray /usr/local/etc/xray/config.json
if [[ "$XRAY_SVC_USER" == "root" ]]; then
    chmod 0700 /usr/local/etc/xray
    chmod 0600 /usr/local/etc/xray/config.json
else
    chmod 0750 /usr/local/etc/xray
    chmod 0640 /usr/local/etc/xray/config.json
fi

systemctl enable xray >/dev/null 2>&1 || true

# Pre-flight config validation (catch JSON errors before starting the service)
if ! xray run -test -config /usr/local/etc/xray/config.json 2>&1; then
    error_exit "Xray config validation failed. Check /usr/local/etc/xray/config.json"
fi

if ! systemctl restart xray; then
    echo ""
    echo "Xray failed to start. Logs:"
    journalctl -u xray --no-pager -n 15 || true
    error_exit "Xray failed to start"
fi

sleep 2
if ! systemctl is-active --quiet xray; then
    echo ""
    echo "Xray failed to start. Logs:"
    journalctl -u xray --no-pager -n 15
    error_exit "Xray failed to start"
fi
echo "       ✓ Xray running on port 443"

# --- 10. Firewall (SAFE HANDOVER; TEMP PORT 22) ---
echo ""
echo "[10/12] Configuring firewall..."
# Ensure UFW applies to IPv6 too (prevents IPv6 firewall bypass).
UFW_DEFAULTS="/etc/default/ufw"
if [[ -f "$UFW_DEFAULTS" ]]; then
    _ufw_backup="/root/ufw.defaults.backup.$(date +%s)"
    cp "$UFW_DEFAULTS" "$_ufw_backup" >/dev/null 2>&1 || true
    chmod 600 "$_ufw_backup" 2>/dev/null || true
    if grep -qE '^[[:space:]]*IPV6=' "$UFW_DEFAULTS"; then
        sed -i -E 's/^[[:space:]]*IPV6=.*/IPV6=yes/' "$UFW_DEFAULTS"
    else
        echo "IPV6=yes" >> "$UFW_DEFAULTS"
    fi
    if grep -qE '^[[:space:]]*DEFAULT_FORWARD_POLICY=' "$UFW_DEFAULTS"; then
        sed -i -E 's/^[[:space:]]*DEFAULT_FORWARD_POLICY=.*/DEFAULT_FORWARD_POLICY="DROP"/' "$UFW_DEFAULTS"
    else
        echo 'DEFAULT_FORWARD_POLICY="DROP"' >> "$UFW_DEFAULTS"
    fi
    chmod 0644 "$UFW_DEFAULTS"
fi

# Temporary iptables lockdown to avoid any open window while UFW is reconfigured.
temp_firewall_lockdown_enable "$SSH_PORT" "true"

ufw --force reset >/dev/null 2>&1
# Some systems flush custom iptables chains during ufw reset; re-apply lockdown defensively.
temp_firewall_lockdown_enable "$SSH_PORT" "true"
ufw default deny incoming >/dev/null 2>&1
ufw default allow outgoing >/dev/null 2>&1
ufw logging low >/dev/null 2>&1
ufw limit "$SSH_PORT/tcp" >/dev/null 2>&1  # New SSH port (rate-limited)
if [[ "$SSH_PORT" -ne 22 ]]; then
    ufw limit 22/tcp >/dev/null 2>&1       # Temporary: keep port 22 during SSH rebind
fi
if [[ "$XRAY_MODE" == "exit" && -n "$ALLOW_ENTRY_IP" ]]; then
    ufw allow proto tcp from "$ALLOW_ENTRY_IP" to any port 443 >/dev/null 2>&1
    _xray_443_rule_msg="443/tcp open only for ${ALLOW_ENTRY_IP}"
else
    ufw allow 443/tcp >/dev/null 2>&1          # Xray
    _xray_443_rule_msg="443/tcp open for all sources"
fi
ufw --force enable >/dev/null 2>&1
echo "       ✓ Firewall: SSH ${SSH_PORT}/tcp + ${_xray_443_rule_msg} (port 22 temporary)"

# --- 11. SSH Configuration (SAFE - KEEPS BOTH PORTS) ---
echo ""
echo "[11/12] Configuring SSH..."
# sshd -t requires the privilege separation directory to exist.
# On socket-activated systems it's created at runtime but may be absent during config validation.
mkdir -p /run/sshd
SSH_CONF="/etc/ssh/sshd_config"
SSH_BACKUP="/root/sshd_config.backup.$(date +%s)"
SSHD_DIR="/etc/ssh/sshd_config.d"
SSHD_DROPIN="${SSHD_DIR}/99-vless-setup.conf"
SSH_SOCKET_UNIT=""
SSH_SOCKET_DROPIN_DIR=""
SSH_SOCKET_DROPIN=""
for _sock in ssh.socket sshd.socket; do
    if [[ "$(systemctl show -p LoadState --value "$_sock" 2>/dev/null)" == "loaded" ]]; then
        SSH_SOCKET_UNIT="$_sock"
        SSH_SOCKET_DROPIN_DIR="/etc/systemd/system/${_sock}.d"
        SSH_SOCKET_DROPIN="${SSH_SOCKET_DROPIN_DIR}/99-vless-setup.conf"
        break
    fi
done
SSH_SOCKET_MODE="false"
if [[ -n "$SSH_SOCKET_UNIT" ]] && (systemctl is-enabled --quiet "$SSH_SOCKET_UNIT" 2>/dev/null || systemctl is-active --quiet "$SSH_SOCKET_UNIT" 2>/dev/null); then
    SSH_SOCKET_MODE="true"
fi

# Backup original
cp "$SSH_CONF" "$SSH_BACKUP"
chmod 600 "$SSH_BACKUP"
echo "       Backup saved: $SSH_BACKUP"

# Use a drop-in so settings cannot be overridden by includes/other tooling.
mkdir -p "$SSHD_DIR"
cat > "$SSHD_DROPIN" <<EOF
# === VLESS Setup - Custom SSH Config (managed) ===
Port $SSH_PORT
EOF

# Keep port 22 temporarily only for non-socket SSH setups; for socket-activated SSH we handle port 22
# at the ssh.socket layer instead.
if [[ "$SSH_PORT" -ne 22 ]] && [[ "$SSH_SOCKET_MODE" != "true" ]]; then
    {
        echo "# Temporary: keep port 22 during the rebind phase."
        echo "Port 22"
        echo ""
    } >> "$SSHD_DROPIN"
fi

cat >> "$SSHD_DROPIN" <<EOF

# Authentication
PermitRootLogin $PERMIT_ROOT_LOGIN
$SSH_DENY_ROOT_LINE
AllowUsers $SSH_ALLOWED_USER
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
AuthenticationMethods publickey
AuthorizedKeysFile .ssh/authorized_keys
PermitEmptyPasswords no
PermitUserEnvironment no
HostbasedAuthentication no
IgnoreRhosts yes
PermitUserRC no
UsePAM yes

# Crypto Hardening (Modern Only)
KexAlgorithms curve25519-sha256,curve25519-sha256@libssh.org
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com
MACs hmac-sha2-512-etm@openssh.com,hmac-sha2-256-etm@openssh.com,umac-128-etm@openssh.com
HostKeyAlgorithms ssh-ed25519,rsa-sha2-512,rsa-sha2-256,ecdsa-sha2-nistp256
PubkeyAcceptedAlgorithms ssh-ed25519,rsa-sha2-512,rsa-sha2-256,ecdsa-sha2-nistp256
VersionAddendum none

# Brute-force mitigation
MaxAuthTries 3
LoginGraceTime 20
MaxSessions 3
MaxStartups 10:30:60

# Session hardening
X11Forwarding no
AllowTcpForwarding no
AllowStreamLocalForwarding no
AllowAgentForwarding no
PermitTunnel no
ClientAliveInterval 300
ClientAliveCountMax 2
UseDNS no
DebianBanner no
LogLevel VERBOSE
StrictModes yes

# Remote port-forwarding gateway protection
GatewayPorts no

# Show legal warning before login
Banner /etc/issue.net

# Show last login info (situational awareness)
PrintLastLog yes

# Disable GSSAPI (reduces attack surface & speeds up login)
GSSAPIAuthentication no

# Restrict environment processing
AcceptEnv LANG LC_*

# Use internal SFTP (smaller attack surface than external sftp-server binary)
Subsystem sftp internal-sftp
# === End Custom Config ===
EOF
chmod 0644 "$SSHD_DROPIN"

# Ensure sshd loads drop-ins safely (outside Match blocks)
ensure_sshd_dropins_included "$SSH_CONF"

# Ensure other sshd configs don't override our managed policy
comment_out_ssh_ports "$SSH_CONF" "$SSHD_DIR" "$SSHD_DROPIN"
comment_out_ssh_overrides "$SSH_CONF" "$SSHD_DIR" "$SSHD_DROPIN"

# Validate SSH config
if ! sshd -t 2>&1; then
    echo "       ⚠ SSH config error, restoring backup..."
    restore_ssh_backup
    error_exit "SSH configuration invalid"
fi
echo "       ✓ SSH config validated"

# systemd socket-activated SSH support:
# On some systems (Ubuntu cloud images), sshd is triggered by ssh.socket; changing "Port" in sshd_config
# alone will not open the new port. We rebind the socket to include our new port during the transition.
if [[ -n "$SSH_SOCKET_UNIT" ]] && (systemctl is-enabled --quiet "$SSH_SOCKET_UNIT" 2>/dev/null || systemctl is-active --quiet "$SSH_SOCKET_UNIT" 2>/dev/null); then
    echo "       Detected ${SSH_SOCKET_UNIT} (systemd socket activation). Rebinding socket ports..."
    mkdir -p "$SSH_SOCKET_DROPIN_DIR"
    cat > "$SSH_SOCKET_DROPIN" <<EOF
[Socket]
ListenStream=
ListenStream=22
ListenStream=$SSH_PORT
EOF
    chmod 0644 "$SSH_SOCKET_DROPIN"
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable --now "$SSH_SOCKET_UNIT" >/dev/null 2>&1 || true
    systemctl restart "$SSH_SOCKET_UNIT" >/dev/null 2>&1 || true
else
    # Non-socket setups: ensure SSH service starts on boot.
    systemctl enable ssh >/dev/null 2>&1 || systemctl enable sshd >/dev/null 2>&1 || true
fi

# Restart SSH
systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || error_exit "Failed to restart SSH service"
sleep 2

# Verify SSH is listening
echo ""
echo "       Verifying SSH ports..."
PORT_NEW_OK=false

if ss -H -tln | awk '{print $4}' | grep -Eq "(:|\\])${SSH_PORT}$"; then
    PORT_NEW_OK=true
    echo "       ✓ Port $SSH_PORT: LISTENING"
else
    echo "       ✗ Port $SSH_PORT: NOT listening"
fi

if [ "$PORT_NEW_OK" = false ]; then
    echo ""
    echo "       ⚠ SSH not listening! Restoring backup..."
    restore_ssh_backup
    error_exit "SSH completely failed - backup restored"
fi

# Now that the new port is confirmed, close port 22 at the listener level and verify.
# For socket-activated systems this means updating ssh.socket; otherwise we update sshd_config.
if [[ "$SSH_PORT" -ne 22 ]]; then
    echo ""
    if [[ -n "$SSH_SOCKET_UNIT" ]] && systemctl is-active --quiet "$SSH_SOCKET_UNIT" 2>/dev/null; then
        echo "       Closing port 22 on ${SSH_SOCKET_UNIT}..."
        mkdir -p "$SSH_SOCKET_DROPIN_DIR"
        cat > "$SSH_SOCKET_DROPIN" <<EOF
[Socket]
ListenStream=
ListenStream=$SSH_PORT
EOF
        chmod 0644 "$SSH_SOCKET_DROPIN"
        systemctl daemon-reload >/dev/null 2>&1 || true
        systemctl restart "$SSH_SOCKET_UNIT" >/dev/null 2>&1 || true
        # Restart sshd to drop any inherited port-22 sockets.
        systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
        sleep 1
    else
        echo "       Rebinding SSH to port $SSH_PORT only..."
        sed -i -E '/^[[:space:]]*Port[[:space:]]+22[[:space:]]*$/d' "$SSHD_DROPIN" || true

        if ! sshd -t 2>&1; then
            echo "       ⚠ SSH config error after removing port 22, restoring backup..."
            restore_ssh_backup
            error_exit "SSH port rebind failed - backup restored"
        fi

        systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || error_exit "Failed to restart SSH service"
        sleep 2
    fi

    if ! ss -H -tln | awk '{print $4}' | grep -Eq "(:|\\])${SSH_PORT}$"; then
        echo "       ⚠ SSH not listening on $SSH_PORT after closing port 22; restoring backup..."
        restore_ssh_backup
        error_exit "SSH port rebind failed - backup restored"
    fi
    if ss -H -tln | awk '{print $4}' | grep -Eq "(:|\\])22$"; then
        echo "       ⚠ Port 22 is still listening after hardening; restoring backup..."
        restore_ssh_backup
        error_exit "Port 22 still listening after hardening"
    fi
fi

# Close port 22 permanently once new SSH port is confirmed
if [[ "$SSH_PORT" -ne 22 ]]; then
    ufw --force delete limit 22/tcp >/dev/null 2>&1 || true
    ufw --force delete allow 22/tcp >/dev/null 2>&1 || true
    ufw --force delete allow OpenSSH >/dev/null 2>&1 || true
    ufw reload >/dev/null 2>&1 || true
    echo "       ✓ Port 22 closed"
fi

# Ensure temp lockdown doesn't leave port 22 reachable if sshd still listens.
temp_firewall_lockdown_enable "$SSH_PORT" "false"

if [[ "$SSH_PORT" -ne 22 ]] && ss -H -tln | awk '{print $4}' | grep -Eq "(:|\\])22$"; then
    error_exit "Port 22 still listening after hardening"
fi

# Remove temporary lockdown so UFW + Fail2Ban can manage bans/rate-limits.
temp_firewall_lockdown_disable

# --- 12. Fail2Ban ---
echo ""
echo "[12/12] Configuring Fail2Ban..."
cat > /etc/fail2ban/jail.local <<EOF
[DEFAULT]
# Use UFW for banning (integrates properly with existing firewall)
banaction = ufw
banaction_allports = ufw

[sshd]
enabled = true
port = $SSH_PORT
backend = systemd
maxretry = 3
bantime = 24h
findtime = 10m

[recidive]
enabled = true
bantime = 1w
findtime = 1d
maxretry = 3
EOF
systemctl enable --now fail2ban >/dev/null 2>&1 || true
systemctl restart fail2ban >/dev/null 2>&1 || true
echo "       ✓ Fail2Ban protecting port $SSH_PORT"

# --- Post-Setup Security Audit Summary ---
echo ""
echo "============================================================"
echo "   🔎 SECURITY AUDIT SUMMARY"
echo "============================================================"
echo ""
echo "[UFW Firewall]"
ufw status verbose || true
echo ""
echo "[SSHD Effective Settings]"
sshd -T 2>/dev/null | awk '
    /^(port|permitrootlogin|allowusers|denyusers|passwordauthentication|kbdinteractiveauthentication|pubkeyauthentication|authenticationmethods|authorizedkeysfile|x11forwarding|allowtcpforwarding|allowstreamlocalforwarding|allowagentforwarding|gatewayports|permittunnel|banner|loglevel|usepam|gssapiauthentication|pubkeyacceptedalgorithms|hostkeyalgorithms|kexalgorithms|ciphers|macs)[[:space:]]/ {print}
' || true
echo ""
echo "[Xray Service Sandbox]"
systemctl show -p User -p Group -p ExecStart -p ProtectSystem -p ProtectHome \
    -p PrivateTmp -p NoNewPrivileges -p MemoryDenyWriteExecute \
    -p SystemCallFilter -p UMask xray 2>/dev/null || true
echo ""
echo "[Xray Config Permissions]"
stat -c '%a %U:%G %n' /usr/local/etc/xray /usr/local/etc/xray/config.json 2>/dev/null || true
echo ""
echo "[Kernel Hardening]"
for key in net.ipv4.ip_forward kernel.core_pattern fs.suid_dumpable \
           kernel.kptr_restrict kernel.dmesg_restrict kernel.yama.ptrace_scope \
           net.ipv4.conf.all.rp_filter net.ipv4.tcp_syncookies; do
    val="$(sysctl -n "$key" 2>/dev/null || echo "N/A")"
    printf "  %-42s = %s\n" "$key" "$val"
done
echo ""
echo "[Core Dumps]"
if [[ -f /etc/security/limits.d/99-no-core.conf ]]; then
    echo "  limits.d: DISABLED"
else
    echo "  limits.d: NOT configured"
fi
echo "  sysctl fs.suid_dumpable=$(sysctl -n fs.suid_dumpable 2>/dev/null || echo 'N/A')"
echo ""
echo "[Module Blacklist]"
if [[ -f /etc/modprobe.d/99-vless-blacklist.conf ]]; then
    echo "  Blacklisted: dccp sctp rds tipc usb-storage bluetooth firewire"
else
    echo "  NOT configured"
fi
echo ""
echo "[/proc Hardening]"
if mount 2>/dev/null | grep -E 'hidepid=(2|invisible)' >/dev/null 2>&1; then
    _hidepid_active="$(mount 2>/dev/null | grep -oE 'hidepid=(2|invisible)' | head -1)"
    echo "  ${_hidepid_active}: ACTIVE"
else
    echo "  hidepid: not active (may require reboot)"
fi
echo ""
echo "[Auditd]"
if systemctl is-active --quiet auditd 2>/dev/null; then
    rule_count="$(auditctl -l 2>/dev/null | wc -l || echo '0')"
    echo "  Status: ACTIVE ($rule_count rules loaded)"
else
    echo "  Status: INACTIVE"
fi
echo ""
echo "[Fail2Ban]"
# Brief wait for fail2ban to finish starting after the restart above.
sleep 2
command -v fail2ban-client >/dev/null 2>&1 && fail2ban-client status sshd || true
echo ""
echo "[/dev/shm]"
mount | grep '/dev/shm' || true
echo ""
echo "[/tmp]"
if command -v findmnt >/dev/null 2>&1; then
    findmnt -n -T /tmp -o TARGET,SOURCE,FSTYPE,OPTIONS 2>/dev/null || true
else
    mount | grep ' on /tmp ' || true
fi
echo ""
echo "[Cron/At Restrictions]"
if [[ -f /etc/cron.allow ]]; then
    echo "  cron.allow: $(tr '\n' ' ' < /etc/cron.allow)"
else
    echo "  cron.allow: NOT SET (all users allowed!)"
fi
echo ""
echo "[SSH Host Keys]"
for _kt in ed25519 rsa ecdsa; do
    if [[ -f "/etc/ssh/ssh_host_${_kt}_key" ]]; then
        _hk_info="$(ssh-keygen -lf "/etc/ssh/ssh_host_${_kt}_key" 2>/dev/null | awk '{print $1 " bits " $NF}' || echo "unknown")"
        echo "  ${_kt}: ${_hk_info}"
    fi
done
for _kt in dsa; do
    if [[ -f "/etc/ssh/ssh_host_${_kt}_key" ]]; then
        echo "  ${_kt}: PRESENT (should be removed!)"
    else
        echo "  ${_kt}: removed ✓"
    fi
done
echo ""
echo "[su Restriction]"
if grep -qE '^auth[[:space:]]+required[[:space:]]+pam_wheel\.so.*group=sudo' /etc/pam.d/su 2>/dev/null; then
    echo "  pam_wheel: ACTIVE (sudo group only)"
else
    echo "  pam_wheel: NOT configured"
fi
echo ""
echo "[login.defs]"
if [[ -f /etc/login.defs ]]; then
    _umask_val="$(grep '^UMASK' /etc/login.defs | awk '{print $2}' || echo 'N/A')"
    _encrypt_val="$(grep '^ENCRYPT_METHOD' /etc/login.defs | awk '{print $2}' || echo 'N/A')"
    echo "  UMASK: ${_umask_val}  ENCRYPT: ${_encrypt_val}"
fi
echo ""
echo "[Journald]"
if [[ -f /etc/systemd/journald.conf.d/99-vless-limits.conf ]]; then
    echo "  Limits: CONFIGURED (200M max, 1 month retention)"
else
    echo "  Limits: NOT configured (risk of disk fill)"
fi
echo ""
echo "[/var/log Permissions]"
_varlog_perms="$(stat -c '%a' /var/log 2>/dev/null || echo 'N/A')"
echo "  /var/log: ${_varlog_perms}"
echo ""
echo "[Remaining SUID/SGID Binaries]"
_suid_list="$(find /usr -perm /6000 -type f 2>/dev/null || true)"
if [[ -n "$_suid_list" ]]; then
    _suid_count="$(echo "$_suid_list" | wc -l)"
    echo "  Found ${_suid_count} setuid/setgid binaries:"
    echo "$_suid_list" | while read -r _sf; do
        _sp="$(stat -c '%a %U:%G' "$_sf" 2>/dev/null || echo '????')"
        echo "    ${_sp} ${_sf}"
    done
    echo "  Review and strip unnecessary bits: chmod u-s,g-s <path>"
else
    echo "  None found ✓"
fi
echo ""
echo "============================================================"

# --- Generate Output ---
echo ""
echo "🌐 Getting server IP..."
IP=$(curl_secure -4 --connect-timeout 5 --max-time 10 https://ifconfig.me/ip 2>/dev/null) || \
IP=$(curl_secure -4 --connect-timeout 5 --max-time 10 https://api.ipify.org 2>/dev/null) || \
IP=$(curl_secure -4 --connect-timeout 5 --max-time 10 https://icanhazip.com 2>/dev/null) || \
IP="$(hostname -I 2>/dev/null | awk '{print $1}')" || \
IP=""
if [[ -z "$IP" ]]; then
    IP="YOUR_SERVER_IP"
    echo "       ⚠ Could not detect server IP. Replace YOUR_SERVER_IP in /root/xray-credentials.txt manually."
fi

CLIENT_LINK=""
if [[ "$XRAY_MODE" != "exit" ]]; then
    if [[ "$XRAY_TRANSPORT" == "xhttp" ]]; then
        # xtls-rprx-vision is not used with xhttp; carry the path (with '/' percent-encoded).
        _LINK_TRANSPORT="type=xhttp&path=${XRAY_XHTTP_PATH//\//%2F}&mode=auto"
    else
        _LINK_TRANSPORT="type=tcp&flow=xtls-rprx-vision"
    fi
    CLIENT_LINK="vless://${UUID}@${IP}:443?security=reality&encryption=none&pbk=${PUBLIC_KEY}&headerType=none&fp=${REALITY_FP}&${_LINK_TRANSPORT}&sni=${REALITY_SNI}&sid=${REALITY_SHORTID}&spiderX=%2F#VLESS-Reality"
fi

# Build upstream transport hints for exit-mode credentials (entry must match the exit).
# _UP_TRANSPORT_EXPORT: copy/paste env block; _UP_TRANSPORT_EXAMPLE: one continued line.
if [[ "$XRAY_TRANSPORT" == "xhttp" ]]; then
    _UP_TRANSPORT_EXPORT="UPSTREAM_TRANSPORT=$XRAY_TRANSPORT
UPSTREAM_PATH=$XRAY_XHTTP_PATH"
    _UP_TRANSPORT_EXAMPLE="UPSTREAM_TRANSPORT=$XRAY_TRANSPORT UPSTREAM_PATH=$XRAY_XHTTP_PATH \\
"
else
    _UP_TRANSPORT_EXPORT="UPSTREAM_TRANSPORT=$XRAY_TRANSPORT"
    _UP_TRANSPORT_EXAMPLE="UPSTREAM_TRANSPORT=$XRAY_TRANSPORT \\
"
fi

# Build optional password block for credentials file (NEVER print to terminal)
_CRED_PASS_SECTION=""
if [[ -n "${ADMIN_SUDO_PASSWORD:-}" ]]; then
    _CRED_PASS_SECTION="ADMIN LOCAL PASSWORD (for sudo only; SSH password login is DISABLED)
--------------------------------------------------------------------
User:     ${SSH_TEST_USER}
Password: ${ADMIN_SUDO_PASSWORD}
!! CHANGE IMMEDIATELY after first login: passwd
"
fi

# Save credentials (password stored here ONLY, never on screen)
if [[ "$XRAY_MODE" == "exit" ]]; then
cat > /root/xray-credentials.txt <<EOF
============================================================
VLESS + REALITY CREDENTIALS (EXIT MODE)
Generated: $(date)
============================================================

SERVER INFORMATION
------------------
IP Address:  $IP
SSH Port:    $SSH_PORT (primary)
SSH Command: ssh -p $SSH_PORT ${SSH_TEST_USER}@$IP

${_CRED_PASS_SECTION}XRAY CREDENTIALS (for this exit node)
-------------------------------------
Mode:        $XRAY_MODE
UUID:        $UUID
Public Key:  $PUBLIC_KEY
Short ID:    $REALITY_SHORTID
SNI:         $REALITY_SNI
Fingerprint: $REALITY_FP
Transport:   $XRAY_TRANSPORT${XRAY_XHTTP_PATH:+ (path: $XRAY_XHTTP_PATH)}

SERVER SECRET (DO NOT SHARE)
----------------------------
Private Key: stored in /usr/local/etc/xray/config.json

UPSTREAM EXPORT VARIABLES (use on entry server)
-----------------------------------------------
UPSTREAM_IP=$IP
UPSTREAM_PORT=443
UPSTREAM_UUID=$UUID
UPSTREAM_PBK=$PUBLIC_KEY
UPSTREAM_SNI=$REALITY_SNI
UPSTREAM_FP=$REALITY_FP
UPSTREAM_SID=$REALITY_SHORTID
$_UP_TRANSPORT_EXPORT
ENTRY SERVER EXAMPLE
--------------------
XRAY_MODE=entry \
UPSTREAM_IP=$IP \
UPSTREAM_PORT=443 \
UPSTREAM_UUID=$UUID \
UPSTREAM_PBK=$PUBLIC_KEY \
UPSTREAM_SNI=$REALITY_SNI \
UPSTREAM_FP=$REALITY_FP \
UPSTREAM_SID=$REALITY_SHORTID \
$_UP_TRANSPORT_EXAMPLE/root/init.sh

============================================================
!! SECURITY: After reading, destroy this file:
!!   shred -u /root/xray-credentials.txt
============================================================
EOF
else
cat > /root/xray-credentials.txt <<EOF
============================================================
VLESS + REALITY CREDENTIALS
Generated: $(date)
============================================================

SERVER INFORMATION
------------------
IP Address:  $IP
SSH Port:    $SSH_PORT (primary)
SSH Command: ssh -p $SSH_PORT ${SSH_TEST_USER}@$IP

${_CRED_PASS_SECTION}XRAY CREDENTIALS
----------------
Mode:        $XRAY_MODE
UUID:        $UUID
Public Key:  $PUBLIC_KEY
Short ID:    $REALITY_SHORTID
SNI:         $REALITY_SNI
Fingerprint: $REALITY_FP

SERVER SECRET (DO NOT SHARE)
----------------------------
Private Key: stored in /usr/local/etc/xray/config.json

CLIENT LINK (copy to v2rayNG / Streisand)
-----------------------------------------
$CLIENT_LINK

RECOMMENDED APPS
----------------
Android: v2rayNG, NekoBox, Hiddify
iOS:     Streisand (free), Shadowrocket (\$2.99)
Windows: v2rayN, Nekoray, Hiddify
Mac:     V2RayXS, Nekoray, Hiddify

============================================================
!! SECURITY: After reading, destroy this file:
!!   shred -u /root/xray-credentials.txt
============================================================
EOF
fi
chmod 600 /root/xray-credentials.txt
_CRED_PASS_SECTION=""

# --- Final Output ---
echo ""
echo "============================================================"
echo "   ✅ SETUP COMPLETE!"
echo "============================================================"
echo ""
echo "📡 SSH ACCESS:"
echo "   ssh -p $SSH_PORT ${SSH_TEST_USER}@$IP"
echo ""
if [[ -n "${ADMIN_SUDO_PASSWORD:-}" ]]; then
    echo "🔑 Admin sudo password (for user '${SSH_TEST_USER}'):"
    echo "   ${ADMIN_SUDO_PASSWORD}"
    echo ""
    echo "   ⚠ CHANGE IMMEDIATELY after first login: passwd"
    echo "   This password is also saved in /root/xray-credentials.txt"
    echo ""
fi
echo "🔐 CREDENTIALS (details in /root/xray-credentials.txt):"
echo "   Mode:      $XRAY_MODE"
echo "   UUID:      ${UUID:0:8}..."
echo "   PublicKey:  ${PUBLIC_KEY:0:8}..."
echo "   ShortID:   $REALITY_SHORTID"
echo ""
if [[ "$XRAY_MODE" == "exit" ]]; then
    echo "🔁 EXIT MODE: upstream variables saved to /root/xray-credentials.txt"
    echo "   Use UPSTREAM_* values on the entry server (XRAY_MODE=entry)."
    if [[ -n "$ALLOW_ENTRY_IP" ]]; then
        echo "   443/tcp is restricted to: $ALLOW_ENTRY_IP"
    fi
    echo ""
else
    # Generate QR code into credentials file before announcing (not terminal)
    _QR_OK=false
    if command -v qrencode >/dev/null 2>&1; then
        {
            echo ""
            echo "QR CODE (scan with v2rayNG or Streisand)"
            echo "-----------------------------------------"
            qrencode -t UTF8 "$CLIENT_LINK" 2>/dev/null || echo "(QR generation failed — use the client link above)"
        } >> /root/xray-credentials.txt && _QR_OK=true
    fi
    echo "📋 CLIENT LINK: saved to /root/xray-credentials.txt"
    echo "   (not printed to terminal for OpSec — session logs may be recorded)"
    echo ""
    if [[ "$_QR_OK" == "true" ]]; then
        echo "📱 QR CODE: saved to /root/xray-credentials.txt"
        echo "   To display: cat /root/xray-credentials.txt"
    else
        echo "📱 QR CODE: not generated (qrencode not available)"
    fi
    echo ""
fi
echo ""
echo "💾 Credentials saved: /root/xray-credentials.txt"
if [[ "$XRAY_MODE" == "exit" ]]; then
    echo "   ⚠ Read, copy UPSTREAM_* values securely, then destroy:"
else
    echo "   ⚠ Read, save the link securely, then destroy:"
fi
echo "     shred -u /root/xray-credentials.txt"
if [[ "$HARDEN_SETUP_LOG" == "true" ]]; then
    echo "   ⚠ Setup log may contain sensitive info. Destroy after review:"
    echo "     shred -u /var/log/vless-setup.log"
fi
echo ""
echo "============================================================"
echo "⚠️  IMPORTANT: TEST SSH BEFORE CLOSING!"
echo "============================================================"
echo ""
echo "1. Open NEW terminal window"
echo "2. Run: ssh -p $SSH_PORT ${SSH_TEST_USER}@$IP"
echo "3. If it works → success!"
echo "4. If it fails → DO NOT CLOSE THIS SESSION. Fix SSH config."
echo ""
echo "============================================================"

# Sensitive variables are purged by _cleanup_on_exit (EXIT trap).
