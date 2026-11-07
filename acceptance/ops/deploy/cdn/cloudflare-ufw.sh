#!/usr/bin/env bash
set -euo pipefail
umask 077

die() { printf 'error: %s\n' "$1" >&2; exit 1; }
[[ ${EUID:-$(id -u)} -eq 0 ]] || die "run as root"

backup_dir="/root/xhttp-backup/ufw-pre-cloudflare"

restore_rules() {
    [[ -f "$backup_dir/user.rules" && -f "$backup_dir/user6.rules" ]] || \
        die "firewall backup is missing at $backup_dir"
    cp -a "$backup_dir/user.rules" /etc/ufw/user.rules
    cp -a "$backup_dir/user6.rules" /etc/ufw/user6.rules
    ufw reload
}

mode="${1:-}"
if [[ "$mode" == "rollback" ]]; then
    restore_rules
    printf 'Restored pre-Cloudflare UFW rules. Verify SSH and port 443 externally.\n'
    exit 0
fi

[[ "$mode" == "apply" ]] || die "usage: $0 apply PUBLIC_DOMAIN | rollback"
PUBLIC_DOMAIN="${2:-}"
[[ -n "$PUBLIC_DOMAIN" ]] || die "PUBLIC_DOMAIN is required"
[[ "$PUBLIC_DOMAIN" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])$ ]] || die "invalid PUBLIC_DOMAIN"
[[ ! -e "$backup_dir" ]] || die "backup already exists; refusing to overwrite the rollback point"

status="$(ufw status verbose)"
grep -Fq 'Status: active' <<< "$status" || die "UFW is not active"
grep -Fq 'Default: deny (incoming)' <<< "$status" || die "UFW incoming default is not deny"
grep -Eq '22222/tcp[[:space:]]' <<< "$status" || die "SSH 22222 rule was not found"

range_file="$(mktemp)"
trap 'rm -f "$range_file"' EXIT
curl -fSs https://www.cloudflare.com/ips-v4 -o "$range_file"

mapfile -t ranges < <(sed '/^[[:space:]]*$/d' "$range_file")
((${#ranges[@]} >= 10)) || die "unexpectedly short Cloudflare IPv4 list"
for cidr in "${ranges[@]}"; do
    [[ "$cidr" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[12][0-9]|3[0-2])$ ]] || \
        die "invalid IPv4 CIDR received"
    ip="${cidr%/*}"
    IFS=. read -r a b c d <<< "$ip"
    for octet in "$a" "$b" "$c" "$d"; do
        ((10#$octet <= 255)) || die "invalid IPv4 CIDR received"
    done
done

install -d -m 0700 "$backup_dir"
cp -a /etc/ufw/user.rules "$backup_dir/user.rules"
cp -a /etc/ufw/user6.rules "$backup_dir/user6.rules"
install -m 0700 "$0" /root/xhttp-backup/cloudflare-ufw.sh

rollback_on_exit() {
    rc=$?
    trap - EXIT
    if ((rc != 0)); then
        printf 'Sealing failed; restoring previous UFW rules.\n' >&2
        restore_rules || printf 'Automatic UFW restore failed; use provider console and %s.\n' \
            /root/xhttp-backup/cloudflare-ufw.sh >&2
    fi
    exit "$rc"
}
trap rollback_on_exit EXIT

for cidr in "${ranges[@]}"; do
    if ! ufw status | grep -Fq "$cidr"; then
        ufw allow proto tcp from "$cidr" to any port 443 comment 'xhttp-cloudflare-v4'
    fi
done

# init.sh creates this exact broad rule. New CDN rules are already present before it is removed.
ufw --force delete allow 443/tcp || true
ufw reload

if ! curl -fSs --http2 --max-time 20 "https://${PUBLIC_DOMAIN}/" >/dev/null; then
    die "Cloudflare reachability test failed"
fi

trap - EXIT
printf 'Origin UFW sealed to Cloudflare IPv4 ranges. Backup: %s\n' "$backup_dir"
printf 'Now verify externally: CDN 443 works; origin-IP 443 and port 22 fail; SSH 22222 works.\n'
