# Cronet: reviewed zlib dictionary bound fix

Linux AMD64 and ARM64 shared libraries from native acceptance run 42637392545.
THM-ZLIB-001 is fixed in both generic and Chromium SIMD inflate implementations.
Each CPU passed eight ASan/UBSan cases, the HTTP range ABI regression and Naive
H2/H3 single/three-hop TCP/UDP checks. This is a prerelease; it does not claim
completion of the broader C/C++ audit or the all-protocol installer acceptance.

Source revisions and trees are in source.tsv; publication.tsv identifies the
wrapper target and SHA-256 of each uncompressed source tar used by the native
build. Complete Git archives and source-tree license/notice texts accompany the
binaries. Inclusion of a notice does not assert that its component is linked.
The wrapper source includes build instructions; the pinned NaiveProxy source
includes the native build scripts and changes. Binary archives retain the native
build provenance and evidence hashes. policy.json binds the reviewed libraries
and native test evidence; SHA256SUMS covers every other release asset.

The project verifier checks the downloaded source contents and notices against
the reviewed native inputs, and the binaries against native acceptance policy:

```sh
just cronet-release-verify-stage PATH_TO_DOWNLOADED_ASSETS
```

No changes to the default installer are implied by publication of this release.
