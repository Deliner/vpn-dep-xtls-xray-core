package owned

import "vpn-script/internal/quality/dependencies/modules"

type Module struct {
	Path     string
	Version  string
	Info     string
	GoMod    string
	Zip      string
	Dir      string
	Sum      string
	GoModSum string
	Origin   *modules.Origin
	Error    string
}

type Record struct {
	Module           Module            `json:"module"`
	Files            map[string]string `json:"file_sha256"`
	RequestDirectory string            `json:"request_directory"`
}
