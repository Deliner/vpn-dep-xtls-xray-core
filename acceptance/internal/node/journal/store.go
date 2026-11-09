package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"vpn-script/internal/platform/safeio"
)

func Load(base string) (*Record, error) {
	path := filepath.Join(base, Name)
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var record Record
	if err := safeio.LoadJSON(path, &record); err != nil {
		return nil, err
	}
	return &record, record.validate(base)
}

// Begin never replaces another transaction, even if its record cannot be decoded.
// The caller owns the install lock throughout Begin, Commit and Clear.
func Begin(base string, record Record) error {
	if record.Phase != "pending" {
		return fmt.Errorf("new transaction must be pending")
	}
	if err := record.validate(base); err != nil {
		return err
	}
	return safeio.NewJSON(filepath.Join(base, Name), record)
}

func Commit(base string, record Record) (Record, error) {
	if record.Phase != "pending" {
		return record, fmt.Errorf("only a pending transaction can commit")
	}
	if err := unchanged(base, record); err != nil {
		return record, err
	}
	record.Phase = "committed"
	return record, safeio.JSON(filepath.Join(base, Name), record)
}

func Clear(base string, record Record) error {
	if err := unchanged(base, record); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(base, Name)); err != nil {
		return err
	}
	return safeio.SyncDir(base)
}

func unchanged(base string, expected Record) error {
	current, err := Load(base)
	if err != nil {
		return err
	}
	if current == nil || !reflect.DeepEqual(*current, expected) {
		return fmt.Errorf("installer transaction changed while holding its lock")
	}
	return nil
}
