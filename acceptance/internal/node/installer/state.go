package installer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

func (i *Installer) current() (string, error) {
	path := filepath.Join(i.Paths.Base, "current")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("managed current path must be a symlink")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if filepath.Dir(target) != filepath.Join(i.Paths.Base, "releases") || !validate.Hash(filepath.Base(target)) {
		return "", fmt.Errorf("unexpected current release target")
	}
	return target, nil
}

func (i *Installer) switchTo(target string) error {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	path := filepath.Join(i.Paths.Base, ".current-"+hex.EncodeToString(random[:]))
	if err := os.Symlink(target, path); err != nil {
		return err
	}
	defer os.Remove(path)
	if err := os.Rename(path, filepath.Join(i.Paths.Base, "current")); err != nil {
		return err
	}
	return safeio.SyncDir(i.Paths.Base)
}

func (i *Installer) state(data model.NodeConfig) (*State, error) {
	var state State
	err := safeio.LoadJSON(filepath.Join(i.Paths.Base, "state.json"), &state)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	current, currentErr := i.current()
	if currentErr != nil {
		return nil, currentErr
	}
	if os.IsNotExist(err) {
		if current != "" {
			return nil, fmt.Errorf("uncommitted release exists; inspect it before retrying")
		}
		return nil, nil
	}
	if state.TopologyID != data.TopologyID || state.NodeID != data.NodeID {
		return nil, fmt.Errorf("machine belongs to another topology/node; explicit migration required")
	}
	if current == "" || !validate.Hash(state.Generation) || filepath.Base(current) != state.releaseID() {
		return nil, fmt.Errorf("interrupted or externally changed deployment; restore committed state before retrying")
	}
	if state.Firewall == "ufw" && data.Firewall != "ufw" {
		return nil, fmt.Errorf("remove managed UFW policy explicitly before selecting preserve")
	}
	return &state, i.verifyCommitted(&state)
}

func (i *Installer) verifyCommitted(state *State) error {
	group, err := i.existingGroup()
	if err != nil {
		return err
	}
	for _, path := range []string{i.Paths.Base, filepath.Join(i.Paths.Base, "releases")} {
		if err := safeio.CheckAccess(path, os.ModeDir|0750, os.Geteuid(), group); err != nil {
			return err
		}
	}
	files := state.Files
	if len(files) == 0 {
		files = map[string]string{"xray": state.BinarySHA256, "config.json": state.ConfigSHA256}
	}
	directory := filepath.Join(i.Paths.Base, "releases", state.releaseID())
	return release.Verify(directory, files, os.Geteuid(), group)
}

func (i *Installer) nextState(data model.NodeConfig, directory string, old Snapshot) ([]byte, error) {
	hashes, err := release.Inventory(directory, data)
	if err != nil {
		return nil, err
	}
	state := State{TopologyID: data.TopologyID, NodeID: data.NodeID, Generation: data.Generation, ReleaseID: filepath.Base(directory),
		Firewall: data.Firewall, Verification: "engine processes and local listeners only; external route verification pending"}
	if data.SchemaVersion == 2 {
		state.Files = hashes
	} else {
		state.ConfigSHA256, state.BinarySHA256 = hashes["config.json"], hashes["xray"]
	}
	if old.Previous != "" {
		var previous State
		if err := json.Unmarshal(old.State, &previous); err != nil {
			return nil, fmt.Errorf("cannot preserve previous release identity: %w", err)
		}
		state.PreviousGeneration, state.PreviousReleaseID = previous.Generation, filepath.Base(old.Previous)
		if directory == old.Previous {
			state.PreviousGeneration, state.PreviousReleaseID = previous.PreviousGeneration, previous.PreviousReleaseID
		}
	}
	return safeio.FormattedJSON(state)
}

func (i *Installer) verifySnapshot(snapshot Snapshot, data model.NodeConfig) error {
	if snapshot.Previous == "" {
		if snapshot.State != nil {
			return fmt.Errorf("transaction state has no committed release")
		}
		return nil
	}
	var state State
	if err := json.Unmarshal(snapshot.State, &state); err != nil {
		return err
	}
	if state.TopologyID != data.TopologyID || state.NodeID != data.NodeID || !validate.Hash(state.Generation) ||
		filepath.Base(snapshot.Previous) != state.releaseID() {
		return fmt.Errorf("transaction snapshot does not identify the owned committed release")
	}
	return i.verifyCommitted(&state)
}
