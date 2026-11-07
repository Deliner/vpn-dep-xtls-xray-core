# N-hop maintained Xray

This branch starts at upstream v26.10.31, commit
`a4a3dff5b20c2faae567b742c172c7b95e75c083`. Upstream licenses and notices remain intact.

The local UDP listener must exclusively own an automatically allocated port.
The upstream listener enabled SO_REUSEPORT even for port zero, allowing a second
listener to share its address. The patch leaves explicitly selected ports and
external socket controllers unchanged. A kernel-backed regression first failed
on the original source and verifies that the second bind is rejected after the fix.

Reviewed dependency updates address the five Go audit records retained by the
official release: Brotli, klauspost/compress, x/net HTML and x/crypto OpenPGP.
These locks match the existing tested N-hop candidate inputs; this is not an
upstream release or a claim that every dependency is already sourced from our forks.

MASQUE H3 intermittent UDP loss is still under investigation. The UDP port fix
does not establish a cause or fix for that separate observation.
