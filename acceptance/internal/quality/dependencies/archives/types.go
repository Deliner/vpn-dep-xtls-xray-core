package archives

import "vpn-script/internal/quality/dependencies/checkout"

type Pin struct {
	Source     checkout.Identity `json:"source"`
	URL        string            `json:"url"`
	SHA256     string            `json:"sha256"`
	Size       int               `json:"size"`
	Files      int               `json:"files"`
	Submodules map[string]string `json:"submodules"`
	Symlinks   map[string]string `json:"symlinks,omitempty"`
}

type fileIdentity struct {
	SHA256     string
	Executable bool
	Gitlink    string
	Symlink    string
}
