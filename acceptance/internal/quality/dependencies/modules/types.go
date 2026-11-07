package modules

type Origin struct {
	VCS    string
	URL    string
	Subdir string
	Hash   string
	Ref    string
}

type Module struct {
	Path     string
	Version  string
	Main     bool
	Indirect bool
	Sum      string
	GoModSum string
	Origin   *Origin
	Replace  *Module
	Error    *struct{ Err string }
}

type Scope struct {
	Name        string   `json:"name"`
	Directory   string   `json:"directory"`
	ManifestSHA string   `json:"manifest_sha256"`
	SumsSHA     string   `json:"sums_sha256"`
	Modules     []Module `json:"modules"`
}

type Report struct {
	Schema int     `json:"schema"`
	Scopes []Scope `json:"scopes"`
}

type Resolution struct {
	Path    string  `json:"path"`
	Version string  `json:"version"`
	Origin  *Origin `json:"origin,omitempty"`
	Failure string  `json:"failure,omitempty"`
}

type OriginBatch struct {
	InventorySHA string       `json:"inventory_sha256"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
	Results      []Resolution `json:"results"`
}
