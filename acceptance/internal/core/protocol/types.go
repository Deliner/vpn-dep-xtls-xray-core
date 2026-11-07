package protocol

type Spec struct {
	ID          string `json:"id"`
	Engine      string `json:"engine,omitempty"`
	TLS         bool   `json:"tls"`
	Donor       bool   `json:"donor"`
	Network     string `json:"listener_network,omitempty"`
	UDP         bool   `json:"payload_udp"`
	Status      string `json:"status"`
	Requirement string `json:"requirement,omitempty"`
}
