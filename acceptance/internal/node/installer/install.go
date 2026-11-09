package installer

import (
	"context"
	"path/filepath"
	"reflect"
	"time"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func New(ctx context.Context) *Installer {
	return &Installer{Context: ctx, Run: process.Run, Sleep: time.Sleep,
		Paths: Paths{Base: "/opt/vpn-nhop", Unit: "/etc/systemd/system/vpn-nhop.service",
			Firewall: "/etc/ufw/applications.d/vpn-nhop"}}
}

func (i *Installer) Install(data model.NodeConfig) (Result, error) {
	result := Result{Node: data.NodeID, Generation: data.Generation, Verification: "local process/listeners; run nhop verify"}
	if err := manifest.Validate(data); err != nil {
		return result, err
	}
	if err := i.preflight(); err != nil {
		return result, err
	}
	lock, err := safeio.Lock(filepath.Join(i.Paths.Base, "install.lock"))
	if err != nil {
		return result, err
	}
	defer lock.Close()
	result.Result, err = i.installLocked(data)
	return result, err
}

func (i *Installer) installLocked(data model.NodeConfig) (string, error) {
	if err := i.recoverTransaction(data); err != nil {
		return "", err
	}
	state, err := i.state(data)
	if err != nil {
		return "", err
	}
	unchanged, err := i.unchanged(state, data)
	if err != nil {
		return "", err
	}
	if unchanged {
		return "unchanged", i.ensureRunning(data)
	}
	if err := i.checkPorts(data); err != nil {
		return "", err
	}
	if err := i.firewallPreflight(data); err != nil {
		return "", err
	}
	arch, err := Architecture()
	if err != nil {
		return "", err
	}
	prepared, err := i.prepare(data, arch)
	if err != nil {
		return "", err
	}
	if err := i.activatePrepared(data, prepared); err != nil {
		return "", err
	}
	return "applied", nil
}

func (i *Installer) unchanged(state *State, data model.NodeConfig) (bool, error) {
	if state == nil || state.Generation != data.Generation {
		return false, nil
	}
	unit, err := optionalFile(i.Paths.Unit)
	if err != nil || string(unit) != unitFor(data) {
		return false, err
	}
	directory := filepath.Join(i.Paths.Base, "releases", state.releaseID())
	hashes, err := release.Inventory(directory, data)
	if err != nil {
		return false, err
	}
	if data.SchemaVersion == 2 {
		self, err := release.SupervisorHash()
		return reflect.DeepEqual(hashes, state.Files) && hashes["nhop-node"] == self, err
	}
	return hashes["config.json"] == state.ConfigSHA256 && hashes["xray"] == state.BinarySHA256, nil
}

func (i *Installer) ensureRunning(data model.NodeConfig) error {
	if err := i.firewallPreflight(data); err != nil {
		return err
	}
	if err := i.systemctl("start"); err != nil {
		return err
	}
	if err := i.ready(data); err != nil {
		return err
	}
	if err := i.applyFirewall(data); err != nil {
		return err
	}
	return i.systemctl("enable")
}
