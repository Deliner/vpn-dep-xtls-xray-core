# Render the existing XHTTP/CDN deployment from its explicit environment file.
[group('legacy')]
cdn-render config='config.env':
    set -a; source "$1"; set +a; bash ops/deploy/cdn/render-xhttp-cloudflare.sh

# MUTATES THIS HOST: run the existing hardening installer with explicit environment settings.
[group('legacy')]
legacy-init:
    bash ops/deploy/legacy/init.sh

# MUTATES THIS HOST: apply the existing CDN UFW profile (run on the intended exit host).
[group('legacy')]
cdn-firewall mode domain='':
    bash ops/deploy/cdn/cloudflare-ufw.sh "$1" "$2"
