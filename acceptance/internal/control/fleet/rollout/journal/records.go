package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func (s *Store) Names() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "plans"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".nhop-") && entry.Type().IsRegular() {
			continue
		}
		name, valid := strings.CutSuffix(entry.Name(), ".json")
		if !valid || !entry.Type().IsRegular() || validate.Name(name) != nil {
			return nil, fmt.Errorf("unexpected rollout journal entry")
		}
		names = append(names, name)
	}
	if len(names) > MaxPlans {
		return nil, fmt.Errorf("rollout journal exceeds plan limit")
	}
	return names, nil
}

func (s *Store) Read(id string) (Record, error) {
	if validate.Name(id) != nil {
		return Record{}, registry.ErrInvalid
	}
	var value Record
	if err := readPrivate(filepath.Join(s.root, "plans", id+".json"), &value); err != nil {
		if os.IsNotExist(err) {
			return Record{}, registry.ErrNotFound
		}
		return Record{}, err
	}
	if value.Spec.ID != id {
		return Record{}, fmt.Errorf("rollout journal identity mismatch")
	}
	return value, ValidateRecord(value)
}

func (s *Store) Save(value Record, create bool) error {
	if err := ValidateRecord(value); err != nil {
		return err
	}
	path := filepath.Join(s.root, "plans", value.Spec.ID+".json")
	if create {
		return safeio.NewJSON(path, value)
	}
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return err
	}
	return safeio.JSON(path, value)
}
