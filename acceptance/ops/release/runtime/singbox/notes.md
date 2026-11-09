# Maintained sing-box 1.14.4 with reviewed Cronet and TUIC shutdown fix

Linux AMD64 and ARM64 engines and native companions tested together in
[native run 42687501127](https://github.com/Deliner/vpn-dep-xtls-xray-core/actions/runs/42687501127).
All 23 profiles passed simultaneous TCP/UDP, IPv4/IPv6, mixed single/three-hop
and failure/recovery checks on both CPUs. Each CPU also passed 80 TUIC shutdown
regressions with the race detector and 240 authenticated remote QUIC closes.
Go source and binary audits passed, including a conservative source scan of the
original sing-quic version without its local replacement.
This is a prerelease: MASQUE remains experimental, the broader C/C++ audit is
not closed, and real VPS/long-soak acceptance is still outstanding.

Each installer-compatible tar.gz contains the tested sing-box and libcronet.so,
the complete tested Go source tree with LICENSE/module locks, and the reviewed
native policy. Cronet includes the THM-ZLIB-001 fix in generic and SIMD inflate.
Its exact full source archives, original license/notice texts and source pins
are separate assets, preserved from the
[reviewed Cronet release](https://github.com/Deliner/vpn-dep-sagernet-cronet-go/releases/tag/nhop-cronet-zlib-1).
Source revisions, trees and uncompressed tar hashes are in cronet-source.tsv
and cronet-publication.tsv. Inclusion of a notice does not assert linkage.

The maintained base commit remains f7e15a404d9d5809b209488a223d830f9307eda8.
The tested source tree includes the complete sing-quic module, its LICENSE and
origin record under nhop-dependencies, with the reviewed service/listener shutdown
patches applied through a relative replacement. No extra dependency fork is used.
Both native source trees contain the same 1,642 files. The accepted run preserves
hidden files directly; each CPU's 1,081 evidence entries and complete runtime/source
inventory were independently verified. The earlier run 42684493775 was rejected
for incomplete report uploads and is not the release input.

This verifies the shutdown regression and concurrent protocols. Installer/Fleet
rollout recovery with these published archives remains a separate acceptance step.

policy.json binds both CPUs to the reviewed native inventories and successful
audits. SHA256SUMS covers every other asset. The project independently checks a
downloaded release with:

```sh
just singbox-release-verify-stage PATH_TO_DOWNLOADED_ASSETS
```

Publishing these assets does not itself change the default installer pins.
