package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"vpn-script/internal/platform/safeio"
)

func protectedParent(path string) error {
	return safeio.ProtectedParent(path, os.Geteuid())
}

func privateRead(path string, value any) error {
	raw, err := privateBytes(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid private Fleet JSON structure")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing private Fleet JSON data")
	}
	return nil
}

func privateBytes(path string) ([]byte, error) {
	if err := protectedParent(path); err != nil {
		return nil, err
	}
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return nil, err
	}
	return safeio.Read(path, 16<<20)
}

func newOutput(path string) error {
	if err := protectedParent(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Fleet output must not already exist: %s", path)
	}
	return nil
}
