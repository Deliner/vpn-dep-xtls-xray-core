package masque

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"time"
)

const socksPort = 10800
const tcpOrigin = "198.18.0.2:18080"
const udpOriginPort = 18081
const exitNamespace = "nhop-masque-exit"

type managed struct {
	command   *exec.Cmd
	done      chan error
	log       *os.File
	stopped   bool
	cleanExit bool
}

type fixture struct {
	directory string
	binary    string
	client    *managed
	server    *managed
	sequence  int
}

type impairment struct {
	Name   string   `json:"name"`
	MTU    int      `json:"mtu"`
	Netem  []string `json:"netem"`
	Policy string   `json:"policy"`
}

type flowResult struct {
	Network   string `json:"network"`
	Worker    int    `json:"worker"`
	Attempted int    `json:"attempted"`
	Sent      int    `json:"sent"`
	Received  int    `json:"received"`
	Duplicate int    `json:"duplicates"`
	Foreign   int    `json:"foreign"`
	Missing   []int  `json:"missing_sequences,omitempty"`
	Error     string `json:"error,omitempty"`
}

type origins struct {
	tcp  net.Listener
	udp  net.PacketConn
	log  *os.File
	done chan error
}

type phaseReport struct {
	Nonce      int                `json:"nonce"`
	Phase      impairment         `json:"phase"`
	Cycle      int                `json:"cycle"`
	Started    time.Time          `json:"started"`
	Finished   time.Time          `json:"finished"`
	DurationMS int64              `json:"duration_ms"`
	Flows      []flowResult       `json:"flows"`
	Before     networkObservation `json:"before"`
	After      networkObservation `json:"after"`
}

type networkObservation struct {
	Client linkObservation `json:"client"`
	Exit   linkObservation `json:"exit"`
}

type linkObservation struct {
	Queue      json.RawMessage `json:"qdisc"`
	Link       json.RawMessage `json:"link"`
	SNMP       string          `json:"snmp"`
	UDPSockets string          `json:"udp_sockets"`
	Drops      uint64          `json:"drops"`
}

type packetID struct {
	Nonce    uint64
	Worker   uint32
	Sequence uint32
}

type queueDrop struct {
	UnixNS   int64
	Protocol byte
	ID       packetID
	HasID    bool
}

type queueEvidence struct {
	Available bool               `json:"available"`
	Total     int                `json:"queue_events"`
	UDP       int                `json:"udp_events"`
	TCP       int                `json:"tcp_events"`
	Phases    []phaseAttribution `json:"phases"`
}

type phaseAttribution struct {
	Nonce       int    `json:"nonce"`
	Cycle       int    `json:"cycle"`
	Name        string `json:"name"`
	Missing     int    `json:"missing_udp"`
	During      int    `json:"queue_drop_during_phase"`
	Late        int    `json:"queue_drop_after_deadline"`
	Unexplained int    `json:"unexplained_by_http3_queue"`
}
