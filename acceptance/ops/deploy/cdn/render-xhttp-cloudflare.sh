#!/usr/bin/env bash
set -euo pipefail
umask 077

die() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

need() {
    local name="$1"
    [[ -n "${!name:-}" ]] || die "$name is required"
}

command -v jq >/dev/null 2>&1 || die "jq is required"

for name in \
    PUBLIC_DOMAIN EXIT_UUID XHTTP_PATH \
    ENTRY_UUID ENTRY_REALITY_PRIVATE_KEY ENTRY_REALITY_SHORT_ID ENTRY_REALITY_SNI; do
    need "$name"
done

XHTTP_LOCAL_PORT="${XHTTP_LOCAL_PORT:-8001}"
XHTTP_PADDING="${XHTTP_PADDING:-100-1000}"
ENTRY_PORT="${ENTRY_PORT:-443}"
ENTRY_REALITY_FP="${ENTRY_REALITY_FP:-chrome}"
CAMOUFLAGE_ROOT="${CAMOUFLAGE_ROOT:-/var/www/camouflage}"
ORIGIN_CERT_FILE="${ORIGIN_CERT_FILE:-/etc/ssl/cloudflare/${PUBLIC_DOMAIN}.pem}"
ORIGIN_KEY_FILE="${ORIGIN_KEY_FILE:-/etc/ssl/cloudflare/${PUBLIC_DOMAIN}.key}"
OUTPUT_DIR="${OUTPUT_DIR:-build/xhttp-cloudflare}"
NGINX_IPV6="${NGINX_IPV6:-false}"
NGINX_HTTP2_STYLE="${NGINX_HTTP2_STYLE:-legacy}"
SSH_ADMIN_USER="${SSH_ADMIN_USER:-secureadmin}"
SSH_IDENTITY_FILE="${SSH_IDENTITY_FILE:-~/.ssh-custom/vps_ed25519}"
SSH_KNOWN_HOSTS_FILE="${SSH_KNOWN_HOSTS_FILE:-~/.ssh-custom/known_hosts}"

[[ "$PUBLIC_DOMAIN" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])$ ]] || \
    die "PUBLIC_DOMAIN must be a hostname"
[[ "$ENTRY_REALITY_SNI" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])$ ]] || \
    die "ENTRY_REALITY_SNI must be a hostname"
[[ "$EXIT_UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || \
    die "EXIT_UUID has an invalid format"
[[ "$ENTRY_UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || \
    die "ENTRY_UUID has an invalid format"
[[ "$ENTRY_REALITY_PRIVATE_KEY" =~ ^[A-Za-z0-9_-]{42,44}=?$ ]] || \
    die "ENTRY_REALITY_PRIVATE_KEY has an invalid format"
[[ "$ENTRY_REALITY_SHORT_ID" =~ ^[0-9a-f]{8,16}$ ]] || \
    die "ENTRY_REALITY_SHORT_ID must be 8-16 lowercase hex characters"
[[ "$XHTTP_PATH" =~ ^/[A-Za-z0-9_-]{16,64}$ ]] || \
    die "XHTTP_PATH must be one random 16-64 character URL-safe path segment"
[[ "$XHTTP_PADDING" =~ ^[0-9]+(-[0-9]+)?$ ]] || \
    die "XHTTP_PADDING must be an integer or min-max range"
padding_min="${XHTTP_PADDING%%-*}"
padding_max="${XHTTP_PADDING##*-}"
((padding_min <= padding_max && padding_max <= 1048576)) || \
    die "XHTTP_PADDING must be ordered and no larger than 1048576"
[[ "$XHTTP_LOCAL_PORT" =~ ^[0-9]+$ ]] && ((XHTTP_LOCAL_PORT >= 1 && XHTTP_LOCAL_PORT <= 65535)) || \
    die "XHTTP_LOCAL_PORT must be in 1..65535"
[[ "$ENTRY_PORT" =~ ^[0-9]+$ ]] && ((ENTRY_PORT >= 1 && ENTRY_PORT <= 65535)) || \
    die "ENTRY_PORT must be in 1..65535"
for path_name in CAMOUFLAGE_ROOT ORIGIN_CERT_FILE ORIGIN_KEY_FILE; do
    [[ "${!path_name}" =~ ^/[A-Za-z0-9._/-]+$ ]] || die "$path_name must be a safe absolute path"
done

case "$NGINX_IPV6" in true|false) ;; *) die "NGINX_IPV6 must be true or false" ;; esac
case "$NGINX_HTTP2_STYLE" in legacy|modern) ;; *) die "NGINX_HTTP2_STYLE must be legacy or modern" ;; esac
[[ "$SSH_ADMIN_USER" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]] || die "SSH_ADMIN_USER is invalid"
for ssh_path_name in SSH_IDENTITY_FILE SSH_KNOWN_HOSTS_FILE; do
    [[ "${!ssh_path_name}" =~ ^(~|/)[A-Za-z0-9._~/-]+$ ]] || die "$ssh_path_name is invalid"
done

case "$ENTRY_REALITY_FP" in
    chrome|firefox|safari|ios|android|edge|360|qq|random|randomized) ;;
    *) die "unsupported ENTRY_REALITY_FP" ;;
esac

render_client=false
if [[ -n "${ENTRY_ADDRESS:-}${ENTRY_REALITY_PUBLIC_KEY:-}" ]]; then
    need ENTRY_ADDRESS
    need ENTRY_REALITY_PUBLIC_KEY
    [[ "$ENTRY_ADDRESS" =~ ^[A-Za-z0-9.-]+$ ]] || die "ENTRY_ADDRESS must be an IPv4 address or hostname"
    [[ "$ENTRY_REALITY_PUBLIC_KEY" =~ ^[A-Za-z0-9_-]{42,44}=?$ ]] || \
        die "ENTRY_REALITY_PUBLIC_KEY has an invalid format"
    command -v xray >/dev/null 2>&1 || die "xray is required to verify keys before creating client artifacts"
    x25519_out="$(xray x25519 -i "$ENTRY_REALITY_PRIVATE_KEY" 2>/dev/null || true)"
    derived_public_key="$({
        awk -F':' '
            tolower($1) ~ /^[[:space:]]*(public|password)/ {
                sub(/^[[:space:]]+/, "", $2)
                sub(/[[:space:]]+$/, "", $2)
                print $2
                exit
            }' <<< "$x25519_out"
    } || true)"
    [[ -n "$derived_public_key" ]] || die "could not derive the public key with xray x25519"
    [[ "$derived_public_key" == "$ENTRY_REALITY_PUBLIC_KEY" ]] || \
        die "ENTRY_REALITY_PUBLIC_KEY does not match ENTRY_REALITY_PRIVATE_KEY"
    render_client=true
fi

upstream_addresses=()
if [[ -n "${CF_EDGE_IPS:-}" ]]; then
    [[ "${ENABLE_CF_POOL:-false}" == "true" ]] || \
        die "CF_EDGE_IPS requires ENABLE_CF_POOL=true after the version gate in the runbook"
    IFS=',' read -r -a raw_addresses <<< "$CF_EDGE_IPS"
    for address in "${raw_addresses[@]}"; do
        address="${address//[[:space:]]/}"
        [[ -n "$address" ]] || continue
        IFS=. read -r a b c d extra <<< "$address"
        [[ -z "${extra:-}" && -n "${d:-}" ]] || die "CF_EDGE_IPS supports IPv4 literals only"
        for octet in "$a" "$b" "$c" "$d"; do
            [[ "$octet" =~ ^[0-9]{1,3}$ ]] && ((10#$octet <= 255)) || \
                die "invalid IPv4 address in CF_EDGE_IPS"
        done
        upstream_addresses+=("$address")
    done
    ((${#upstream_addresses[@]} > 0)) || die "CF_EDGE_IPS did not contain an address"
else
    upstream_addresses+=("$PUBLIC_DOMAIN")
fi

addresses_json="$(printf '%s\n' "${upstream_addresses[@]}" | jq -R . | jq -s .)"

install -d -m 0700 "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR/client-link.txt" "$OUTPUT_DIR/entry-probe.json" "$OUTPUT_DIR/handoff.txt"

jq -n \
    --arg uuid "$EXIT_UUID" \
    --arg path "$XHTTP_PATH" \
    --arg padding "$XHTTP_PADDING" \
    --argjson port "$XHTTP_LOCAL_PORT" \
    '{
      log: {loglevel: "warning"},
      inbounds: [{
        tag: "xhttp-in",
        listen: "127.0.0.1",
        port: $port,
        protocol: "vless",
        settings: {
          clients: [{id: $uuid}],
          decryption: "none"
        },
        streamSettings: {
          network: "xhttp",
          security: "none",
          xhttpSettings: {
            path: $path,
            mode: "packet-up",
            extra: {xPaddingBytes: $padding}
          }
        }
      }],
      routing: {
        domainStrategy: "AsIs",
        rules: [{type: "field", ip: ["geoip:private"], outboundTag: "block"}]
      },
      outbounds: [
        {protocol: "freedom", tag: "direct"},
        {protocol: "blackhole", tag: "block"}
      ]
    }' > "$OUTPUT_DIR/exit-xray.json"

jq -n \
    --arg entry_uuid "$ENTRY_UUID" \
    --arg entry_private_key "$ENTRY_REALITY_PRIVATE_KEY" \
    --arg entry_short_id "$ENTRY_REALITY_SHORT_ID" \
    --arg entry_sni "$ENTRY_REALITY_SNI" \
    --arg entry_fp "$ENTRY_REALITY_FP" \
    --arg exit_uuid "$EXIT_UUID" \
    --arg public_domain "$PUBLIC_DOMAIN" \
    --arg xhttp_path "$XHTTP_PATH" \
    --arg padding "$XHTTP_PADDING" \
    --argjson entry_port "$ENTRY_PORT" \
    --argjson addresses "$addresses_json" \
    '
    def upstream($tag; $address): {
      protocol: "vless",
      tag: $tag,
      settings: {
        vnext: [{
          address: $address,
          port: 443,
          users: [{id: $exit_uuid, encryption: "none", flow: ""}]
        }]
      },
      streamSettings: {
        network: "xhttp",
        security: "tls",
        tlsSettings: {
          serverName: $public_domain,
          alpn: ["h2", "http/1.1"],
          fingerprint: "chrome"
        },
        xhttpSettings: {
          host: $public_domain,
          path: $xhttp_path,
          mode: "packet-up",
          extra: {xPaddingBytes: $padding}
        }
      }
    };

    (if ($addresses | length) == 1 then
       [upstream("upstream"; $addresses[0])]
     else
       [$addresses | to_entries[] | upstream(("cf" + ((.key + 1) | tostring)); .value)]
     end) as $upstreams |

    ({
      log: {loglevel: "warning"},
      inbounds: [{
        tag: "reality-in",
        port: $entry_port,
        protocol: "vless",
        settings: {
          clients: [{id: $entry_uuid, flow: "xtls-rprx-vision"}],
          decryption: "none"
        },
        streamSettings: {
          network: "tcp",
          security: "reality",
          realitySettings: {
            dest: ($entry_sni + ":443"),
            serverNames: [$entry_sni],
            privateKey: $entry_private_key,
            shortIds: [$entry_short_id],
            fingerprint: $entry_fp
          }
        },
        sniffing: {
          enabled: true,
          destOverride: ["http", "tls", "quic"],
          routeOnly: true
        }
      }],
      routing: {
        domainStrategy: "AsIs",
        rules: (
          [{type: "field", ip: ["geoip:private"], outboundTag: "block"}] +
          [if ($addresses | length) == 1 then
             {type: "field", network: "tcp,udp", outboundTag: "upstream"}
           else
             {type: "field", network: "tcp,udp", balancerTag: "cfbal"}
           end]
        )
      },
      outbounds: ($upstreams + [{protocol: "blackhole", tag: "block"}])
    }) +
    (if ($addresses | length) == 1 then {}
     else {
       observatory: {
         subjectSelector: ["cf"],
         probeURL: "https://www.gstatic.com/generate_204",
         probeInterval: "20s",
         enableConcurrency: true
       },
       routing: {
         domainStrategy: "AsIs",
         balancers: [{tag: "cfbal", selector: ["cf"], strategy: {type: "leastPing"}}],
         rules: [
           {type: "field", ip: ["geoip:private"], outboundTag: "block"},
           {type: "field", network: "tcp,udp", balancerTag: "cfbal"}
         ]
       }
     } end)
    ' > "$OUTPUT_DIR/entry-xray.json"

jq '.inbounds = [{
      tag: "probe-socks",
      listen: "127.0.0.1",
      port: 10808,
      protocol: "socks",
      settings: {auth: "noauth", udp: true}
    }]' "$OUTPUT_DIR/entry-xray.json" > "$OUTPUT_DIR/upstream-probe.json"

if [[ "$NGINX_HTTP2_STYLE" == "modern" ]]; then
    nginx_listen="listen 443 ssl;"
    nginx_http2="    http2 on;"
else
    nginx_listen="listen 443 ssl http2;"
    nginx_http2=""
fi
nginx_ipv6=""
if [[ "$NGINX_IPV6" == "true" ]]; then
    if [[ "$NGINX_HTTP2_STYLE" == "modern" ]]; then
        nginx_ipv6="    listen [::]:443 ssl;"
    else
        nginx_ipv6="    listen [::]:443 ssl http2;"
    fi
fi

cat > "$OUTPUT_DIR/nginx-xhttp.conf" <<EOF
server {
    ${nginx_listen}
${nginx_ipv6}
${nginx_http2}
    server_name ${PUBLIC_DOMAIN};

    ssl_certificate ${ORIGIN_CERT_FILE};
    ssl_certificate_key ${ORIGIN_KEY_FILE};
    ssl_protocols TLSv1.2 TLSv1.3;

    root ${CAMOUFLAGE_ROOT};
    index index.html;

    location / {
        try_files \$uri \$uri/ =404;
    }

    location ~ ^${XHTTP_PATH}(?:/|$) {
        proxy_pass http://127.0.0.1:${XHTTP_LOCAL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_buffering off;
        proxy_request_buffering off;
        client_max_body_size 0;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
EOF

if [[ "$render_client" == "true" ]]; then
    printf 'vless://%s@%s:%s?security=reality&encryption=none&pbk=%s&headerType=none&fp=%s&type=tcp&flow=xtls-rprx-vision&sni=%s&sid=%s&spiderX=%%2F#VLESS-Entry\n' \
        "$ENTRY_UUID" "$ENTRY_ADDRESS" "$ENTRY_PORT" "$ENTRY_REALITY_PUBLIC_KEY" \
        "$ENTRY_REALITY_FP" "$ENTRY_REALITY_SNI" "$ENTRY_REALITY_SHORT_ID" \
        > "$OUTPUT_DIR/client-link.txt"
    {
        printf 'SSH:\nssh -i %s -o IdentitiesOnly=yes -o UserKnownHostsFile=%s -p 22222 %s@%s\n\n' \
            "$SSH_IDENTITY_FILE" "$SSH_KNOWN_HOSTS_FILE" "$SSH_ADMIN_USER" "$ENTRY_ADDRESS"
        printf 'VLESS:\n'
        cat "$OUTPUT_DIR/client-link.txt"
    } > "$OUTPUT_DIR/handoff.txt"

    jq -n \
        --arg uuid "$ENTRY_UUID" --arg address "$ENTRY_ADDRESS" --argjson port "$ENTRY_PORT" \
        --arg pbk "$ENTRY_REALITY_PUBLIC_KEY" --arg sni "$ENTRY_REALITY_SNI" \
        --arg sid "$ENTRY_REALITY_SHORT_ID" --arg fp "$ENTRY_REALITY_FP" \
        '{log:{loglevel:"warning"},inbounds:[{listen:"127.0.0.1",port:10809,protocol:"socks",settings:{auth:"noauth",udp:true}}],outbounds:[{protocol:"vless",tag:"entry",settings:{vnext:[{address:$address,port:$port,users:[{id:$uuid,encryption:"none",flow:"xtls-rprx-vision"}]}]},streamSettings:{network:"tcp",security:"reality",realitySettings:{serverName:$sni,fingerprint:$fp,publicKey:$pbk,shortId:$sid,spiderX:"/"}}}]}' \
        > "$OUTPUT_DIR/entry-probe.json"
fi

chmod 0600 "$OUTPUT_DIR"/*
printf 'Rendered private deployment artifacts in %s\n' "$OUTPUT_DIR"
printf 'Do not commit or share the rendered directory.\n'
