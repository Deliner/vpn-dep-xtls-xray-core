package packettrace

import (
	"fmt"
	"os"
	"time"
)

// Snapshot copies all observed data while the caller still owns the buffer.
// An admission log may run after transfer and must not inspect that buffer.
func nhopServerQueueSnapshot(tunnel, packet any, data []byte) string {
	network, header := "other", ""
	if value, ok := nhopTCPHeader(data); ok {
		network, header = "tcp", value
	} else if value, ok := nhopUDPHeader(data); ok {
		network, header = "udp", value
	}
	return fmt.Sprintf("pid=%d tunnel=%p packet=%p bytes=%d network=%s %s", os.Getpid(), tunnel, packet, len(data), network, header)
}

func nhopTraceServerQueue(phase, snapshot string, queued, limit int, failed bool) {
	fmt.Fprintf(os.Stderr, "NHOP_SERVER_QUEUE unix_ns=%d phase=%s queued=%d limit=%d failed=%t %s\n", time.Now().UnixNano(), phase, queued, limit, failed, snapshot)
}
