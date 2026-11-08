package boot

import (
	"fmt"
	"os"

	"vpn-script/internal/platform/safeio"
)

func (t transaction) read() (record, bool, error) {
	if _, err := os.Lstat(t.state); os.IsNotExist(err) {
		return record{}, false, nil
	}
	if err := safeio.CheckAccess(t.state, 0600, t.uid, t.gid); err != nil {
		return record{}, false, err
	}
	var value record
	if err := safeio.LoadJSON(t.state, &value); err != nil {
		return value, false, err
	}
	if value.SchemaVersion != 1 || value.OwnerToken != t.snapshot.OwnerToken || value.Generation != t.snapshot.Generation || !validManager(value.NetworkManager) {
		return value, false, fmt.Errorf("invalid or foreign boot registration identity")
	}
	if value.Phase != "enabling" && value.Phase != "enabled" && value.Phase != "disabling" {
		return value, false, fmt.Errorf("unknown boot registration phase")
	}
	return value, true, nil
}

func (t transaction) write(value record, create bool) error {
	if create {
		return safeio.NewJSON(t.state, value)
	}
	return safeio.JSON(t.state, value)
}
