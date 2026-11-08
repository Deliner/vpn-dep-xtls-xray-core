package contract

type PeerMetrics struct {
	DirectDNS  uint64            `json:"direct_dns"`
	TunnelDNS  uint64            `json:"tunnel_dns"`
	DNSQueries map[string]uint64 `json:"dns_queries"`
	Canary     map[string]uint64 `json:"canary"`
	HTTPPeers  map[string]uint64 `json:"http_peers"`
	UDPPeers   map[string]uint64 `json:"udp_peers"`
}

type Origin struct {
	Peer    string `json:"peer"`
	Payload string `json:"payload"`
}

type Canary struct {
	BootID   string `json:"boot_id"`
	Phase    string `json:"phase"`
	Attempts uint64 `json:"attempts"`
	Accepted uint64 `json:"accepted"`
	Started  string `json:"started"`
	Finished string `json:"finished"`
}

type Recovery struct {
	Phase        string            `json:"phase"`
	BeforeBootID string            `json:"before_boot_id"`
	BootID       string            `json:"boot_id"`
	Status       string            `json:"status"`
	Error        string            `json:"error,omitempty"`
	CleanupError string            `json:"cleanup_error,omitempty"`
	Observations map[string]string `json:"observations"`
}
