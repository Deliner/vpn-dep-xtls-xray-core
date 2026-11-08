package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func (s *store) current() ([]byte, error) {
	if _, err := os.Lstat(s.unit); os.IsNotExist(err) {
		return nil, nil
	}
	if err := s.access(s.unit, 0644); err != nil {
		return nil, err
	}
	data, err := safeio.Read(s.unit, 65536)
	if err != nil {
		return nil, err
	}
	line, _, found := strings.Cut(string(data), "\n")
	prefix := marker + s.owner.Token + " "
	id, owned := strings.CutPrefix(line, prefix)
	if !found || !owned || !validate.Hash(id) {
		return nil, fmt.Errorf("refusing foreign client unit: %s", s.unit)
	}
	expected, err := s.generation(id)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, expected) {
		return nil, fmt.Errorf("installed client unit changed; inspect before retrying")
	}
	return data, nil
}

func (s *store) generation(id string) ([]byte, error) {
	if !validate.Hash(id) {
		return nil, fmt.Errorf("invalid client generation")
	}
	directory := filepath.Join(s.base, "releases", id)
	if err := s.access(directory, os.ModeDir|0700); err != nil {
		return nil, err
	}
	if err := s.members(directory); err != nil {
		return nil, err
	}
	var value release
	if err := safeio.LoadJSON(filepath.Join(directory, "release.json"), &value); err != nil {
		return nil, err
	}
	digest, err := safeio.Digest(value.Identity)
	if err != nil || digest != id || value.Identity.SchemaVersion != 1 || value.Identity.Network.EngineUID != 0 ||
		network.ValidateSettings(value.Identity.Network) != nil || len(value.Identity.Hashes) != 5 || !validate.Hash(value.UnitHash) {
		return nil, fmt.Errorf("invalid client generation identity")
	}
	if err := verifyContent(directory, filepath.Join(Directory, "releases", id), value); err != nil {
		return nil, err
	}
	unit, err := safeio.Read(filepath.Join(directory, "unit"), 65536)
	if err != nil || safeio.Hash(unit) != value.UnitHash || !bytes.HasPrefix(unit, []byte(marker+s.owner.Token+" "+id+"\n")) {
		return nil, fmt.Errorf("client generation unit changed")
	}
	return unit, nil
}

func (s *store) members(directory string) error {
	modes := map[string]os.FileMode{"nhop-client": 0755, "xray": 0755, "sing-box": 0755, "libcronet.so": 0755,
		"profile.json": 0600, "runtimes.json": 0600, "release.json": 0600, "unit": 0600}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) != len(modes) {
		return fmt.Errorf("client generation contains missing or unexpected files")
	}
	for _, entry := range entries {
		mode, ok := modes[entry.Name()]
		if !ok {
			return fmt.Errorf("unexpected client generation member")
		}
		if err := s.access(filepath.Join(directory, entry.Name()), mode); err != nil {
			return err
		}
	}
	return nil
}

func verifyContent(directory, destination string, value release) error {
	for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so", "profile.json"} {
		digest, err := safeio.FileHash(filepath.Join(directory, name))
		if err != nil || !validate.Hash(value.Identity.Hashes[name]) || digest != value.Identity.Hashes[name] {
			return fmt.Errorf("%s generation snapshot changed", name)
		}
	}
	if _, err := profile.Load(filepath.Join(directory, "profile.json")); err != nil {
		return err
	}
	expected, err := safeio.FormattedJSON(runtimeInventory(destination, value.Identity))
	if err != nil {
		return err
	}
	actual, err := safeio.Read(filepath.Join(directory, "runtimes.json"), 65536)
	if err != nil || !bytes.Equal(actual, expected) {
		return fmt.Errorf("client runtime inventory changed")
	}
	return nil
}
