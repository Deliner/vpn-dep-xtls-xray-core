package fleet

import (
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// Initialize is an explicit bootstrap operation. Apply never reconstructs a
// missing journal, because doing so could reset the rollback protection.
func (a *Agent) Initialize() error {
	if err := a.trust.Validate(); err != nil {
		return err
	}
	if err := os.Mkdir(a.directory, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
		if err := a.checkDirectory(); err != nil {
			return err
		}
		_, err := a.loadState()
		return err
	}
	if err := a.saveState(a.initialState()); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(a.directory))
}

func (a *Agent) checkDirectory() error {
	return safeio.CheckAccess(a.directory, os.ModeDir|0700, os.Geteuid(), -1)
}

func (a *Agent) loadState() (state, error) {
	path := filepath.Join(a.directory, "state.json")
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return state{}, err
	}
	var value state
	if err := safeio.LoadJSON(path, &value); err != nil {
		return state{}, err
	}
	if err := a.validateState(value); err != nil {
		return state{}, err
	}
	return value, nil
}

func (a *Agent) saveState(value state) error {
	if err := a.validateState(value); err != nil {
		return err
	}
	return safeio.JSON(filepath.Join(a.directory, "state.json"), value)
}
