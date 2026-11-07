# Deployment-specific instructions

Local legacy installer: `ops/deploy/legacy/init.sh`; `/root/init.sh` below is the remote copy.
Use root `AGENTS.md` and `just --list` for development commands.

Single exit: run XRAY_MODE=exit /root/init.sh, then use generated credentials from /root/xray-credentials.txt.

  Entry + exit: setup exit first, copy UPSTREAM_*, then run entry with XRAY_MODE=entry UPSTREAM_*... /root/init.sh.

  From existing VLESS exit link: parse host/port/uuid/pbk/sni/fp/sid and pass them as UPSTREAM_* to entry.

  Provider A/normal VPS: default script flow is OK.

  Provider B/Google-style images: use HARDEN_NORMALIZE_AUTHORIZED_KEYS=true HARDEN_ADMIN_USER=<real_user>.

  Use SSH keys and known-hosts under ~/.ssh-custom, not default ~/.ssh.

  Final output must include SSH connect command and VLESS string.

  Always verify: SSH on the configured port, expected obsolete ports closed, and client IP equals exit IP.
  Legacy init.sh defaults: SSH 22222 and port 22 closed. For the Go nhop CLI use each node's
  ssh.port and ssh.closed_ports from YAML; do not hardcode 22222 in verification. VPN listener
  ports and per-route client.socks_port are configurable independently. For the CDN topology,
  entry Xray owns :443; exit Nginx owns :443; exit Xray owns only 127.0.0.1:8001.

  ## DPI-evasion notes (2026)

  Transport: default XRAY_TRANSPORT=tcp (REALITY + xtls-rprx-vision, covers DPI layers 1-3) —
  this is the verified-working, recommended transport for a bare single-host REALITY listener.
  XRAY_TRANSPORT=xhttp is opt-in for layer-4/behavioral-ML resistance (the primary anti-VLESS
  vector since Dec 2025) and adds XHTTP + xPaddingBytes padding (vision flow is dropped, as it
  is incompatible); entry/exit must share the transport (pass UPSTREAM_TRANSPORT + UPSTREAM_PATH).
  ⚠ KNOWN LIMITATION (tested 2026-06-25 on v25.1.1 AND v26.3.27): the script's `xhttp` config is
  schema-valid and REALITY auth succeeds, but a DIRECT single-host REALITY+XHTTP does NOT carry
  traffic — the inner HTTP/2 stream EOFs (all modes: auto/packet-up/stream-up/stream-one).
  XHTTP is designed to run BEHIND a real web server / CDN ("steal from yourself": real Nginx on
  :443, XRay behind it via grpc_pass/reverse-proxy) — that, or a CDN, is the way to use xhttp.
  XHTTP rules to remember: client+server xray versions MUST match exactly (else it silently
  breaks); XHTTP+REALITY defaults to mode stream-one; Sing-box clients can't do XHTTP (use
  xray-based clients — v2rayN/v2rayNG). For now prefer tcp+vision unless you have Nginx/CDN fronting.

  Camouflage SNI: avoid mass-scanned ("burned") donors (microsoft.com, apple/icloud, cloudflare,
  amazon, bing). Pick a less-common site co-located in the VPS datacenter (low ping, TLS1.3,
  HTTP/2, no homepage redirect) via RealiTLScanner against the VPS IP; the script warns on burned
  defaults. Fingerprint: keep REALITY_FP=chrome (android/ios are weaker/buggy).

  Mobile / whitelist era: mobile operators allow only ~0.14% of RU IPs; Yandex.Cloud (~20% of the
  whitelist, SNI ya.ru) is effectively unblockable. For mobile, host the ENTRY node on Yandex.Cloud
  with REALITY_SNI=ya.ru, then relay to an overseas exit.

  Exit hoster selection: avoid mass-scanned ranges (Hetzner/OVH/DigitalOcean/AWS/Aeza); whole-hoster
  collateral blocks have occurred (Beget/Selectel/TimeWeb, Jun 2026).

  Multi-hop caveat: entry→exit is NOT end-to-end encrypted — the entry relay decrypts in RAM to
  route, so a compromised/observed entry sees traffic. Control both nodes for sensitive use.

  Reverse proxy (bridge/portal): XRay-native `reverse` defeats one-directional whitelist DPI —
  when you can't connect TO the foreign exit but it can connect to your white-IP host, invert it:
  the foreign exit (bridge) dials out and holds a connection to your white-IP host (portal);
  clients hit the portal and exit via the bridge. Reverse-direction traffic is usually unfiltered.
  Useful fallback when the entry hoster itself runs ТСПУ. (Not yet implemented in init.sh.)

  Domestic CDN: a fronting CDN need not be foreign — a Russian/whitelisted CDN (cf. Yandex.Cloud)
  is a valid front and harder to block.
