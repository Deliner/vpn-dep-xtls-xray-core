package model

const Reality = "vless-reality"
const Shadowsocks = "shadowsocks-2022"
const SSMethod = "2022-blake3-aes-256-gcm"
const ClientProfilesVersion = 2

type Definition struct {
	Version int               `json:"version" yaml:"version"`
	Name    string            `json:"name" yaml:"name"`
	Nodes   map[string]*Node  `json:"nodes" yaml:"nodes"`
	Routes  map[string]*Route `json:"routes" yaml:"routes"`
}

type SSH struct {
	User           string `json:"user" yaml:"user"`
	Port           int    `json:"port" yaml:"port"`
	ClosedPorts    []int  `json:"closed_ports" yaml:"closed_ports"`
	IdentityFile   string `json:"identity_file" yaml:"identity_file"`
	KnownHostsFile string `json:"known_hosts_file" yaml:"known_hosts_file"`
}

type Node struct {
	BridgePortStart int                 `json:"bridge_port_start,omitempty" yaml:"bridge_port_start,omitempty"`
	Address         string              `json:"address" yaml:"address"`
	SSH             SSH                 `json:"ssh" yaml:"ssh"`
	Firewall        string              `json:"firewall" yaml:"firewall"`
	Services        map[string]*Service `json:"services" yaml:"services"`
}

type Service struct {
	TLS        TLSFiles `json:"tls,omitempty" yaml:"tls,omitempty"`
	Path       string   `json:"path,omitempty" yaml:"path,omitempty"`
	Protocol   string   `json:"protocol" yaml:"protocol"`
	Port       int      `json:"port" yaml:"port"`
	Listen     string   `json:"listen" yaml:"listen"`
	ServerName string   `json:"server_name,omitempty" yaml:"server_name,omitempty"`
	Target     string   `json:"target,omitempty" yaml:"target,omitempty"`
}

type Client struct {
	SOCKSPort int `json:"socks_port" yaml:"socks_port"`
}

type Route struct {
	Path           []string `json:"path" yaml:"path"`
	ExpectedExitIP string   `json:"expected_exit_ip,omitempty" yaml:"expected_exit_ip,omitempty"`
	Client         Client   `json:"client" yaml:"client"`
}

type Endpoint struct {
	Node    string
	ID      string
	Service Service
}

type Topology struct {
	Definition
	Services   map[string]Endpoint
	Successors map[string]string
	Order      []string
}

type Secret struct {
	EncryptionSeed   string `json:"encryption_seed,omitempty"`
	EncryptionKey    string `json:"encryption_key,omitempty"`
	BridgePassword   string `json:"bridge_password,omitempty"`
	ObfsPassword     string `json:"obfs_password,omitempty"`
	Certificate      string `json:"certificate,omitempty"`
	CertificateKey   string `json:"certificate_key,omitempty"`
	TrustCertificate string `json:"trust_certificate,omitempty"`
	Protocol         string `json:"protocol"`
	Password         string `json:"password,omitempty"`
	UUID             string `json:"uuid,omitempty"`
	ShortID          string `json:"short_id,omitempty"`
	PrivateKey       string `json:"private_key,omitempty"`
	PublicKey        string `json:"public_key,omitempty"`
}

type Secrets struct {
	SchemaVersion int               `json:"schema_version"`
	TopologyID    string            `json:"topology_id"`
	Services      map[string]Secret `json:"services"`
}

type Xray struct {
	Log       Object   `json:"log"`
	Inbounds  []Object `json:"inbounds"`
	Outbounds []Object `json:"outbounds"`
	Routing   Object   `json:"routing"`
}

type Listener struct {
	Internal bool   `json:"internal,omitempty"`
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Network  string `json:"network"`
}

type NodeConfig struct {
	Programs      []Program  `json:"programs,omitempty"`
	SchemaVersion int        `json:"schema_version"`
	TopologyID    string     `json:"topology_id"`
	NodeID        string     `json:"node_id"`
	Runtime       Runtime    `json:"runtime"`
	Firewall      string     `json:"firewall"`
	Listeners     []Listener `json:"listeners"`
	Xray          Xray       `json:"xray"`
	Generation    string     `json:"generation,omitempty"`
}

type Binary struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type DeployNode struct {
	Address string `json:"address"`
	SSH     SSH    `json:"ssh"`
	SHA256  string `json:"sha256"`
}

type Deployment struct {
	SchemaVersion  int                   `json:"schema_version"`
	TopologyID     string                `json:"topology_id"`
	Order          []string              `json:"order"`
	Installers     map[string]Binary     `json:"installers"`
	Nodes          map[string]DeployNode `json:"nodes"`
	ProfilesSHA256 string                `json:"profiles_sha256"`
}

type Profile struct {
	EntryEndpoint  ClientEndpoint `json:"entry_endpoint,omitempty"`
	Engine         string         `json:"engine,omitempty"`
	RuntimeVersion string         `json:"runtime_version,omitempty"`
	ImportNote     string         `json:"import_note,omitempty"`
	URI            string         `json:"uri"`
	Path           []string       `json:"path"`
	HopCount       int            `json:"hop_count"`
	EntryNode      string         `json:"entry_node"`
	ExitNode       string         `json:"exit_node"`
	ExpectedExitIP string         `json:"expected_exit_ip,omitempty"`
	ConfigSHA256   string         `json:"client_config_sha256"`
	SOCKSPort      int            `json:"client_socks_port"`
	UDP443         string         `json:"udp443"`
	DNSPolicy      string         `json:"client_dns_policy"`
}

type ClientEndpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Network string `json:"network"`
}

type Profiles struct {
	SchemaVersion int                `json:"schema_version"`
	TopologyID    string             `json:"topology_id"`
	Routes        map[string]Profile `json:"routes"`
}
