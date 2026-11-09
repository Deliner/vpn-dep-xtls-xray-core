package netem

import (
	"encoding/json"
	"time"
)

// Result distinguishes a reproduced control failure from a failed experiment.
type Result struct {
	Round, TCP, ExpectedTCP, UDP, ExpectedUDP, DelayFailures, Packets int
	Policy                                                            string
}

type flow struct {
	Network   string `json:"network"`
	Worker    int    `json:"worker"`
	Attempted int    `json:"attempted"`
	Sent      int    `json:"sent"`
	Received  int    `json:"received"`
	Duplicate int    `json:"duplicates"`
	Foreign   int    `json:"foreign"`
	Missing   []int  `json:"missing_sequences"`
	Error     string `json:"error"`
}

type impairment struct {
	Name, Policy string
	MTU          int
	Netem        []string
}

type phase struct {
	Nonce, Cycle int
	Phase        impairment
	Started      time.Time
	Finished     time.Time
	DurationMS   int64 `json:"duration_ms"`
	Flows        []flow
	Before       observation
	After        observation
}

type observation struct{ Client, Exit link }

type link struct {
	Queue      json.RawMessage `json:"qdisc"`
	Interface  json.RawMessage `json:"link"`
	UDPSockets string          `json:"udp_sockets"`
	Drops      uint64
}

type experiment struct {
	root, policy string
	inputs       map[string]string
	result       Result
}

type credentialNode struct {
	Protocol       string
	Settings       struct{ Users []struct{ Pass string } }
	StreamSettings struct {
		MasqueSettings struct{ Pass string }
		TLSSettings    struct {
			Certificates []struct{ Certificate []string }
		}
	}
}
