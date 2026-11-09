# Maintained sing-box 1.14.4 with reviewed Cronet

Linux AMD64 and ARM64 engines and native companions tested together in
[native run 42665559172](https://github.com/Deliner/vpn-dep-xtls-xray-core/actions/runs/42665559172).
All 23 profiles passed simultaneous TCP/UDP, IPv4/IPv6, mixed single/three-hop
and failure/recovery checks on both CPUs. Go source and binary audits passed.
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

Native candidate uploads originally omitted 31 hidden source files per CPU.
They were recovered from the retained, checksum-pinned source archive, each
matching the native build inventory. The complete recovered source/runtime
inventory is verified before packaging; original CI artifacts remain unchanged.

policy.json binds both CPUs to the reviewed native inventories and successful
audits. SHA256SUMS covers every other asset. The project independently checks a
downloaded release with:

```sh
just singbox-release-verify-stage PATH_TO_DOWNLOADED_ASSETS
```

Publishing these assets does not itself change the default installer pins.
