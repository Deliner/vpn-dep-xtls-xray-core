package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func (i *Installer) snapshot() (Snapshot, error) {
	var old Snapshot
	var err error
	old.Previous, err = i.current()
	if err != nil {
		return old, err
	}
	old.Unit, err = optionalFile(i.Paths.Unit)
	if err != nil {
		return old, err
	}
	old.Firewall, err = optionalFile(i.Paths.Firewall)
	if err != nil {
		return old, err
	}
	old.State, err = optionalFile(filepath.Join(i.Paths.Base, "state.json"))
	if err != nil {
		return old, err
	}
	old.Active = i.probe("systemctl", "is-active", "--quiet", service)
	old.Enabled = i.probe("systemctl", "is-enabled", "--quiet", service)
	return old, nil
}

func (i *Installer) rollback(old Snapshot, firewallChanged bool) error {
	i.probe("systemctl", "stop", service)
	if old.Previous == "" {
		if err := restoreFile(filepath.Join(i.Paths.Base, "current"), nil, 0); err != nil {
			return err
		}
	} else if err := i.switchTo(old.Previous); err != nil {
		return err
	}
	if !old.Enabled {
		i.probe("systemctl", "disable", service)
	}
	if err := restoreFile(i.Paths.Unit, old.Unit, 0644); err != nil {
		return err
	}
	if err := i.reload(); err != nil {
		return err
	}
	if err := i.restoreFirewall(old.Firewall, firewallChanged); err != nil {
		return err
	}
	if err := restoreFile(filepath.Join(i.Paths.Base, "state.json"), old.State, 0600); err != nil {
		return err
	}
	if old.Active && old.Previous != "" {
		return i.systemctl("restart")
	}
	return nil
}

func (i *Installer) activate(data model.NodeConfig, release string) error {
	old, err := i.snapshot()
	if err != nil {
		return err
	}
	firewallChanged, err := i.apply(data, release, old)
	if err == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	recovery := *i
	recovery.Context = ctx
	if rollbackErr := recovery.rollback(old, firewallChanged); rollbackErr != nil {
		return fmt.Errorf("apply failed and rollback also failed; inspect this node: %w", rollbackErr)
	}
	return err
}

func (i *Installer) activatePrepared(data model.NodeConfig, prepared preparedRelease) error {
	if err := i.activate(data, prepared.path); err != nil {
		// Only this attempt's new, inactive release may be removed. Reused history
		// and a release left current by a failed rollback must remain inspectable.
		if current, checkErr := i.current(); prepared.created && checkErr == nil && current != prepared.path {
			_ = os.RemoveAll(prepared.path)
		}
		return err
	}
	return nil
}

func (i *Installer) apply(data model.NodeConfig, release string, old Snapshot) (bool, error) {
	if err := safeio.Atomic(i.Paths.Unit, []byte(unitFor(data)), 0644); err != nil {
		return false, err
	}
	if err := i.switchTo(release); err != nil {
		return false, err
	}
	if err := i.reload(); err != nil {
		return false, err
	}
	if err := i.systemctl("restart"); err != nil {
		return false, err
	}
	if err := i.ready(data); err != nil {
		return false, err
	}
	changed := data.Firewall == "ufw"
	if err := i.applyFirewall(data); err != nil {
		return changed, err
	}
	if err := i.systemctl("enable"); err != nil {
		return changed, err
	}
	return changed, i.commit(data, release, old)
}
