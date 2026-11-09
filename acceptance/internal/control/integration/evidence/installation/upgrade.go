package installation

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

type Revision struct {
	State     installer.State
	Directory string
	Unit      []byte
}

func UpgradeSupervisor(previous, binary string) (string, error) {
	sha, err := safeio.FileHash(binary)
	if err != nil {
		return "", err
	}
	if previous == "" || sha == previous {
		return "", fmt.Errorf("upgrade requires different previous/current installer bytes")
	}
	return sha, nil
}

func VerifyUpgrade(before, after Revision, old model.NodeConfig, supervisor string) error {
	if before.Directory == after.Directory || after.State.PreviousReleaseID != filepath.Base(before.Directory) ||
		after.State.PreviousGeneration != old.Generation || after.State.Files["nhop-node"] != supervisor {
		return fmt.Errorf("upgrade lost release history or kept the previous supervisor")
	}
	retained, err := release.Inventory(before.Directory, old)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(retained, before.State.Files) {
		return fmt.Errorf("upgrade modified the retained previous release")
	}
	return nil
}

// RetainRuntimes closes the old interval before current switches and rebinds
// observations to the retained release for verification after the new interval.
func RetainRuntimes(runtimes map[string]evidence.Runtime, directory string) error {
	if err := evidence.Verify(runtimes); err != nil {
		return err
	}
	for name, observed := range runtimes {
		_, engine, found := strings.Cut(name, "/")
		if !found || engine != "xray" && engine != "sing-box" {
			return fmt.Errorf("unknown historical runtime role")
		}
		observed.Path = filepath.Join(directory, engine)
		runtimes[name] = observed
	}
	return evidence.Verify(runtimes)
}

func VerifyRuntimeHistory(current, previous map[string]evidence.Runtime) error {
	err := evidence.Verify(current)
	if len(previous) != 0 {
		err = errors.Join(err, evidence.Verify(previous))
	}
	return err
}

func RuntimeTransition(previous, current map[string]evidence.Runtime) error {
	if len(previous) != 4 || len(current) != 4 {
		return fmt.Errorf("runtime transition requires both client/server identities")
	}
	changed := false
	for _, role := range []string{"client/xray", "server/xray", "client/sing-box", "server/sing-box"} {
		old, found := previous[role]
		next, exists := current[role]
		if !found || !exists || !validate.Hash(old.SHA256) || !validate.Hash(next.SHA256) || old.Error != "" || next.Error != "" {
			return fmt.Errorf("incomplete runtime transition identity")
		}
		changed = changed || old.SHA256 != next.SHA256
	}
	if !changed {
		return fmt.Errorf("runtime transition retained every old executable")
	}
	return nil
}
