package fleet

import (
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func (a *Agent) Status() (Status, error) {
	if err := a.checkDirectory(); err != nil {
		return Status{}, err
	}
	lock, err := safeio.Lock(filepath.Join(a.directory, "agent.lock"))
	if err != nil {
		return Status{}, err
	}
	defer lock.Close()
	value, err := a.loadState()
	return value.status(), err
}
