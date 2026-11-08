// Package install publishes private client generations and a manual-start unit.
// It never starts/stops services, changes networking, or enables boot activation.
package install

import (
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/session"
	systemmanager "vpn-script/internal/control/client/systemd/manager"
)

const Service = "nhop-client.service"
const Directory = "/var/lib/nhop-client"
const UnitPath = "/etc/systemd/system/" + Service
const BootStatePath = Directory + "/boot.json"
const properties = "LoadState,ActiveState,SubState,FragmentPath,DropInPaths,UnitFileState,Job"
const marker = "# nhop-client installation v1 "

type Options = controller.Options

// Receipt contains no credentials or source paths. Snapshots remain available
// after unregistering the unit, including for explicit reinstall/rollback.
type Receipt struct {
	Generation string `json:"generation"`
	Unit       string `json:"unit"`
	Directory  string `json:"directory"`
}

type identity struct {
	SchemaVersion int               `json:"schema_version"`
	Network       network.Options   `json:"network"`
	Hashes        map[string]string `json:"hashes"`
}

type release struct {
	Identity identity `json:"identity"`
	UnitHash string   `json:"unit_sha256"`
}

type bundle struct {
	identity identity
	files    map[string][]byte
}

type ownership struct {
	SchemaVersion int    `json:"schema_version"`
	Token         string `json:"token"`
}

type store struct {
	base, unit string
	uid, gid   int
	owner      ownership
	lock       *os.File
}

type manager = systemmanager.Command

type transaction struct {
	store   *store
	manager manager
}

func runtimeInventory(directory string, value identity) controller.Runtimes {
	return controller.Runtimes{SchemaVersion: 1,
		Xray:    session.Artifact{Path: filepath.Join(directory, "xray"), SHA256: value.Hashes["xray"]},
		SingBox: session.Artifact{Path: filepath.Join(directory, "sing-box"), SHA256: value.Hashes["sing-box"]},
		Cronet:  session.Artifact{Path: filepath.Join(directory, "libcronet.so"), SHA256: value.Hashes["libcronet.so"]}}
}
