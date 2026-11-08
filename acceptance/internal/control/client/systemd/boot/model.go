// Package boot owns persistent activation records and systemd dependencies.
// Enable/disable never start the VPN, stop the network manager or release a guard.
package boot

import (
	"vpn-script/internal/control/client/systemd/install"
	"vpn-script/internal/control/client/systemd/manager"
)

const dropIn = "90-nhop-boot.conf"
const properties = "LoadState,ActiveState,SubState,FragmentPath,DropInPaths,UnitFileState,Job,Requires,After"

type Receipt struct {
	Generation     string `json:"generation"`
	NetworkManager string `json:"network_manager"`
	Enabled        bool   `json:"enabled"`
}

type record struct {
	SchemaVersion  int    `json:"schema_version"`
	OwnerToken     string `json:"owner_token"`
	Generation     string `json:"generation"`
	NetworkManager string `json:"network_manager"`
	Phase          string `json:"phase"`
}

type artifact struct {
	path, link string
	data       []byte
}

type transaction struct {
	snapshot *install.Snapshot
	state    string
	units    string
	uid, gid int
	command  manager.Command
}

func validManager(value string) bool {
	return value == "systemd-networkd.service" || value == "NetworkManager.service"
}
