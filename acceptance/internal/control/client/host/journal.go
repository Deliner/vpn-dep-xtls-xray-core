package host

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"vpn-script/internal/platform/safeio"
)

type current struct {
	SchemaVersion int    `json:"schema_version"`
	Session       string `json:"session"`
}

// Next reserves a journal name under the already locked root-owned state tree.
// Session creates its directory and writes its request before network mutation.
func (o *Owner) Next() (string, error) {
	if err := directory(filepath.Join(Directory, "sessions"), 0711); err != nil {
		return "", err
	}
	previous, err := safeio.Read(filepath.Join(Directory, "current.json"), 4096)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	value := current{SchemaVersion: 1, Session: hex.EncodeToString(token[:])}
	if err := safeio.JSON(filepath.Join(Directory, "current.json"), value); err != nil {
		return "", err
	}
	o.previous = previous
	return filepath.Join(Directory, "sessions", value.Session), nil
}

func (o *Owner) ForgetUnstarted() error {
	if o.previous != nil {
		return safeio.Atomic(filepath.Join(Directory, "current.json"), o.previous, 0600)
	}
	return os.Remove(filepath.Join(Directory, "current.json"))
}

func (o *Owner) Current() (string, error) {
	path := filepath.Join(Directory, "current.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "", nil
	}
	if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
		return "", err
	}
	var value current
	if err := safeio.LoadJSON(path, &value); err != nil {
		return "", err
	}
	if value.SchemaVersion != 1 || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(value.Session) {
		return "", fmt.Errorf("invalid client session journal identity")
	}
	path = filepath.Join(Directory, "sessions", value.Session)
	if err := protectedParents(path, 0); err != nil {
		return "", err
	}
	return path, nil
}

// ReadSession checks the protected tree before opening credentials or status.
func ReadSession(directory, name string, value any) error {
	if name != "status.json" && name != "request.json" {
		return fmt.Errorf("unexpected client journal member")
	}
	if err := protectedParents(directory, 0); err != nil {
		return err
	}
	path := filepath.Join(directory, name)
	if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
		return err
	}
	return safeio.LoadJSON(path, value)
}
