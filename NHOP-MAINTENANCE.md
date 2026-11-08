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

CONNECT-IP opts into a per-stream HTTP/3 receive budget of 1024 packets and
1 MiB of payload using the exact owned quic-go commit
`037c5a9142ac658960f28313d54c8607c030a1e1`. The client sets it before request headers;
the proxy sets it before address assignment. Setup failure cancels both directions.
The lazy ring has independent packet and byte bounds; other HTTP/3 streams keep
the upstream 32-packet default. HTTP/2 capsule queues and legacy WARP draft
connection-level datagrams are not enlarged. No packet logging is shipped.

The previous 32-packet stream queue drops the 33rd item in a deterministic
256-packet delayed-consumer regression; the bounded queue accepts that burst.
Both real CONNECT-IP directions also retain 256 distinct 900-byte datagrams while
the application reader is delayed. The full `transport/internet/masque/...` and
`proxy/masque` suites pass with the race detector on Darwin ARM64. This validates
the opt-in API wiring and preserves the HTTP/2, WARP and stream lifecycle tests.

Controlled Linux fault acceptance remains separate. Underlying QUIC, kernel and
proxy queues remain finite, and DATAGRAM transport is unreliable. This patch does
not claim to fix MTU reductions, all loss under latency/load, or the original rare
all-protocol MASQUE failure. The profile remains experimental until those open
observations have their own reproducible regression and verified correction.

A separate `TestNHOPRemoteTCPReordering` connects two real remote TCP stacks
through an in-memory IPv4 link (MTU 1280), without QUIC or HTTP/3. Three batches
of eight 32 KiB echoes use a constant 20 ms scheduling delay; three more use
15..25 ms delays. Timer delivery can reorder even the constant-delay case.
The test measures actual within-flow reorder events and requires observations
in every jitter batch. All 48 exchanges completed within their original two-second
budgets with the race detector on Darwin ARM64. This passing isolation control
does not reproduce or fix the separate Linux MASQUE delay failure.
