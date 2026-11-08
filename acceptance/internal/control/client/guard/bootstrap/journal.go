package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"vpn-script/internal/platform/safeio"
)

func (s *store) read() (journal, bool, error) {
	path := filepath.Join(s.base, "state.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return journal{}, false, nil
	}
	if err := safeio.CheckAccess(path, 0600, s.uid, s.gid); err != nil {
		return journal{}, false, err
	}
	var value journal
	if err := safeio.LoadJSON(path, &value); err != nil {
		return value, false, err
	}
	if value.SchemaVersion != 1 || !regexp.MustCompile(`^nhop:[a-f0-9]{32}$`).MatchString(value.Guard.Token) {
		return value, false, fmt.Errorf("invalid bootstrap journal identity")
	}
	return value, true, nil
}

func (s *store) write(value journal) error {
	if s.lock == nil {
		return fmt.Errorf("bootstrap mutation requires its owned locked state")
	}
	return safeio.JSON(filepath.Join(s.base, "state.json"), value)
}
