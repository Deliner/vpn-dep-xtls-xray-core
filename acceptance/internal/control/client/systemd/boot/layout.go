package boot

import (
	"fmt"
	"path/filepath"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/control/client/systemd/install"
)

func (t transaction) artifacts(value record) ([]artifact, error) {
	data, err := systemd.BootUnit(filepath.Join(t.snapshot.Directory, "nhop-client"))
	if err != nil {
		return nil, err
	}
	marker := fmt.Sprintf("# nhop-client boot v1 %s %s\n", value.OwnerToken, value.Generation)
	gate := []byte(marker + systemd.BootRequirement)
	return []artifact{
		{path: filepath.Join(t.units, systemd.BootService), data: append([]byte(marker), data...)},
		{path: filepath.Join(t.units, "systemd-networkd.service.d", dropIn), data: gate},
		{path: filepath.Join(t.units, "NetworkManager.service.d", dropIn), data: gate},
		{path: filepath.Join(t.units, install.Service+".d", dropIn), data: gate},
		{path: filepath.Join(t.units, "multi-user.target.wants", install.Service), link: "../" + install.Service},
	}, nil
}

func (t transaction) receipt(value record, enabled bool) Receipt {
	return Receipt{Generation: value.Generation, NetworkManager: value.NetworkManager, Enabled: enabled}
}

func (t transaction) noArtifacts() error {
	value := record{OwnerToken: t.snapshot.OwnerToken, Generation: t.snapshot.Generation}
	files, err := t.artifacts(value)
	if err != nil {
		return err
	}
	return t.inspect(files, "absent")
}
