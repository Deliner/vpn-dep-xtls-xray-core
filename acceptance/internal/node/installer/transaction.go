package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/journal"
	"vpn-script/internal/platform/safeio"
)

func (i *Installer) snapshot() (Snapshot, error) {
	old, err := i.snapshotFiles()
	if err != nil {
		return old, err
	}
	old.Active, old.Enabled, err = i.serviceState()
	return old, err
}

func (i *Installer) snapshotFiles() (Snapshot, error) {
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
	return old, nil
}

func (i *Installer) rollback(old Snapshot, firewallChanged bool) error {
	if err := i.stopForRollback(); err != nil {
		return err
	}
	if old.Previous == "" {
		if err := restoreFile(filepath.Join(i.Paths.Base, "current"), nil, 0); err != nil {
			return err
		}
	} else if err := i.switchTo(old.Previous); err != nil {
		return err
	}
	if !old.Enabled {
		if err := i.disableIfPresent(); err != nil {
			return err
		}
	}
	if err := restoreFile(i.Paths.Unit, old.Unit, 0644); err != nil {
		return err
	}
	if err := i.reload(); err != nil {
		return err
	}
	if old.Enabled {
		if err := i.systemctl("enable"); err != nil {
			return err
		}
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
	state, err := i.nextState(data, release, old)
	if err != nil {
		return err
	}
	record := journal.Record{Schema: 1, Phase: "pending", TopologyID: data.TopologyID, NodeID: data.NodeID,
		FirewallChanged: data.Firewall == "ufw", Old: old,
		Next: Snapshot{Previous: release, Unit: []byte(unitFor(data)), State: state, Firewall: old.Firewall, Active: true, Enabled: true}}
	if record.FirewallChanged {
		record.Next.Firewall = []byte(UFWProfile(data))
	}
	if err := journal.Begin(i.Paths.Base, record); err != nil {
		return err
	}
	changed, err := i.apply(data, release, record.Next)
	if err != nil {
		return i.failedTransaction(record, changed, err)
	}
	committed, err := journal.Commit(i.Paths.Base, record)
	if err != nil {
		return err
	}
	return journal.Clear(i.Paths.Base, committed)
}

func (i *Installer) failedTransaction(record journal.Record, changed bool, applyErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	recovery := *i
	recovery.Context = ctx
	if rollbackErr := recovery.rollback(record.Old, changed); rollbackErr != nil {
		return fmt.Errorf("apply failed and rollback also failed; inspect this node: %w", rollbackErr)
	}
	if err := journal.Clear(i.Paths.Base, record); err != nil {
		return fmt.Errorf("rollback completed but transaction cleanup failed: %w", err)
	}
	return applyErr
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

func (i *Installer) apply(data model.NodeConfig, release string, next Snapshot) (bool, error) {
	if err := safeio.Atomic(i.Paths.Unit, next.Unit, 0644); err != nil {
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
	return changed, safeio.Atomic(filepath.Join(i.Paths.Base, "state.json"), next.State, 0600)
}

func (i *Installer) recoverTransaction(data model.NodeConfig) error {
	record, err := journal.Load(i.Paths.Base)
	if err != nil || record == nil {
		return err
	}
	if err := i.verifyTransaction(*record, data); err != nil {
		return err
	}
	if record.Phase == "pending" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		recovery := *i
		recovery.Context = ctx
		if err := recovery.rollback(record.Old, record.FirewallChanged); err != nil {
			return fmt.Errorf("interrupted transaction recovery failed: %w", err)
		}
	}
	return journal.Clear(i.Paths.Base, *record)
}

func (i *Installer) verifyTransaction(record journal.Record, data model.NodeConfig) error {
	if record.TopologyID != data.TopologyID || record.NodeID != data.NodeID {
		return fmt.Errorf("unfinished transaction belongs to another topology/node")
	}
	current, err := i.snapshotFiles()
	if err != nil {
		return err
	}
	if current.Previous != record.Old.Previous && current.Previous != record.Next.Previous ||
		!bytes.Equal(current.Unit, record.Old.Unit) && !bytes.Equal(current.Unit, record.Next.Unit) ||
		!bytes.Equal(current.Firewall, record.Old.Firewall) && !bytes.Equal(current.Firewall, record.Next.Firewall) ||
		!bytes.Equal(current.State, record.Old.State) && !bytes.Equal(current.State, record.Next.State) {
		return fmt.Errorf("deployment changed outside the interrupted transaction; inspect before retrying")
	}
	if record.Phase == "committed" {
		if current.Previous != record.Next.Previous || !bytes.Equal(current.Unit, record.Next.Unit) ||
			!bytes.Equal(current.State, record.Next.State) || !bytes.Equal(current.Firewall, record.Next.Firewall) {
			return fmt.Errorf("committed transaction no longer matches installed state")
		}
		return i.verifySnapshot(record.Next, data)
	}
	return i.verifySnapshot(record.Old, data)
}

func (i *Installer) stopForRollback() error {
	unit, err := optionalFile(i.Paths.Unit)
	if err != nil || unit == nil {
		return err
	}
	if err := i.reload(); err != nil {
		return err
	}
	return i.systemctl("stop")
}
