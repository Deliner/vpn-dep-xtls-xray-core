package install

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

type serviceManager struct {
	unit, loaded string
	state        map[string]string
	calls        []string
	failReloads  int
}

func (m *serviceManager) run(ctx context.Context, args ...string) ([]byte, error) {
	m.calls = append(m.calls, strings.Join(args, " "))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if args[0] == "daemon-reload" {
		if m.failReloads > 0 {
			m.failReloads--
			return nil, errors.New("injected reload failure")
		}
		m.loaded = ""
		if _, err := os.Lstat(m.unit); err == nil {
			m.loaded = m.unit
		}
		return nil, nil
	}
	value := map[string]string{"LoadState": "not-found", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "", "DropInPaths": "", "UnitFileState": "", "Job": ""}
	if m.loaded != "" {
		value["LoadState"], value["FragmentPath"], value["UnitFileState"] = "loaded", m.loaded, "static"
	}
	for key, item := range m.state {
		value[key] = item
	}
	var output []string
	for _, key := range strings.Split(properties, ",") {
		output = append(output, key+"="+value[key])
	}
	return []byte(strings.Join(output, "\n") + "\n"), nil
}

func installation(t *testing.T) (transaction, *serviceManager, bundle) {
	t.Helper()
	root := t.TempDir()
	s, err := openStore(filepath.Join(root, "client"), filepath.Join(root, Service))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.lock.Close() })
	m := &serviceManager{unit: s.unit, state: map[string]string{}}
	options := inputFiles(t)
	b, err := loadBundle(options, elfBytes(1))
	if err != nil {
		t.Fatal(err)
	}
	return transaction{store: s, manager: m.run}, m, b
}

func inputFiles(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	options := Options{Profile: filepath.Join(root, "profile.json"), Runtimes: filepath.Join(root, "runtimes.json"), Network: network.Defaults(0)}
	if err := safeio.NewJSON(options.Profile, importedProfile(t)); err != nil {
		t.Fatal(err)
	}
	artifacts := make([]session.Artifact, 3)
	for i, name := range []string{"xray", "sing-box", "libcronet.so"} {
		data := elfBytes(byte(i + 2))
		path := filepath.Join(root, name)
		if err := safeio.Create(path, data, 0755); err != nil {
			t.Fatal(err)
		}
		artifacts[i] = session.Artifact{Path: path, SHA256: safeio.Hash(data)}
	}
	value := controller.Runtimes{SchemaVersion: 1, Xray: artifacts[0], SingBox: artifacts[1], Cronet: artifacts[2]}
	if err := safeio.NewJSON(options.Runtimes, value); err != nil {
		t.Fatal(err)
	}
	return options
}

func importedProfile(t *testing.T) profile.Imported {
	t.Helper()
	service := model.Service{Protocol: "shadowsocks-2022", Port: 24443}
	top := &model.Topology{Definition: model.Definition{Name: "fixture", Nodes: map[string]*model.Node{"entry": {Address: "198.51.100.7"}},
		Routes: map[string]*model.Route{"main": {Path: []string{"entry.proxy"}, Client: model.Client{SOCKSPort: 12080}}}},
		Services: map[string]model.Endpoint{"entry.proxy": {Node: "entry", ID: "proxy", Service: service}}}
	secret, err := credentials.NewSecret(service.Protocol)
	if err != nil {
		t.Fatal(err)
	}
	config, metadata := engines.Client(top, "main", model.Secrets{Services: map[string]model.Secret{"entry.proxy": secret}})
	metadata.ConfigSHA256, err = safeio.Digest(config)
	if err != nil {
		t.Fatal(err)
	}
	return profile.Imported{SchemaVersion: 1, Topology: "fixture", Route: "main", Metadata: metadata, Config: config}
}

// A minimal ELF header is sufficient for input architecture checks. It is never
// executed; actual installed binaries are exercised separately in the Linux lab.
func elfBytes(suffix byte) []byte {
	data := make([]byte, 65)
	copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(data[16:], 3)
	machine := uint16(62)
	if runtime.GOARCH == "arm64" {
		machine = 183
	}
	binary.LittleEndian.PutUint16(data[18:], machine)
	binary.LittleEndian.PutUint32(data[20:], 1)
	binary.LittleEndian.PutUint16(data[52:], 64)
	data[64] = suffix
	return data
}

func requireApply(t *testing.T, tr transaction, b bundle) Receipt {
	t.Helper()
	r, err := tr.apply(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func readUnit(t *testing.T, tr transaction) []byte {
	t.Helper()
	data, err := os.ReadFile(tr.store.unit)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func kernelCleanup(t *testing.T) {
	t.Helper()
	tr, err := openTransaction()
	if err != nil {
		t.Error("cannot inspect installation fixture for cleanup", err)
		return
	}
	defer tr.store.lock.Close()
	if err := tr.remove(context.Background()); err != nil {
		t.Error("refusing cleanup of uncertain installation state", err)
		return
	}
	if _, err := os.Lstat(UnitPath + ".d"); err == nil {
		if err := os.Remove(UnitPath + ".d"); err != nil {
			t.Error("retaining nonempty fixture drop-in directory", err)
			return
		}
	}
	if err := removeFixtureGenerations(tr.store); err != nil {
		t.Error("refusing cleanup of unverified generation", err)
		return
	}
	for _, name := range []string{"releases", "owner.json", "install.lock", "."} {
		if err := os.Remove(filepath.Join(Directory, name)); err != nil {
			t.Error("cannot remove empty owned installation fixture", err)
			return
		}
	}
}

func removeFixtureGenerations(s *store) error {
	entries, err := os.ReadDir(filepath.Join(s.base, "releases"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := s.generation(entry.Name()); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		directory := filepath.Join(s.base, "releases", entry.Name())
		for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so", "profile.json", "runtimes.json", "release.json", "unit", "."} {
			if err := os.Remove(filepath.Join(directory, name)); err != nil {
				return fmt.Errorf("fixture generation cleanup: %w", err)
			}
		}
	}
	return nil
}
