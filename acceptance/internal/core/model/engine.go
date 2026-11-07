package model

// TLSFiles are controller-local inputs. Their contents are embedded in private bundles.
type TLSFiles struct {
	Certificate string `json:"certificate_file,omitempty" yaml:"certificate_file,omitempty"`
	Key         string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
	CA          string `json:"ca_file,omitempty" yaml:"ca_file,omitempty"`
}

// Program is a pinned, named engine, never an arbitrary shell command.
type Program struct {
	Name    string  `json:"name"`
	Runtime Runtime `json:"runtime"`
	Config  Object  `json:"config"`
}

type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Runtime struct {
	Version string           `json:"version"`
	Assets  map[string]Asset `json:"assets"`
}

type Object map[string]any
