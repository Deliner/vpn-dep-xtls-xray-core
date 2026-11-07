package bundle_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func fakeELF(t *testing.T, directory, name string, machine uint16) {
	t.Helper()
	raw := make([]byte, 64)
	copy(raw, []byte{'\x7f', 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(raw[16:], 2)
	binary.LittleEndian.PutUint16(raw[18:], machine)
	binary.LittleEndian.PutUint32(raw[20:], 1)
	binary.LittleEndian.PutUint16(raw[52:], 64)
	must(t, os.WriteFile(filepath.Join(directory, name), raw, 0700))
}

func fixture(t *testing.T) (*model.Topology, string, string) {
	t.Helper()
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	must(t, err)
	output, installers := filepath.Join(t.TempDir(), "bundle"), fakeInstallers(t)
	must(t, bundle.Render(top, output, installers))
	return top, output, installers
}

func fakeInstallers(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	fakeELF(t, directory, "nhop-node-linux-amd64", 62)
	fakeELF(t, directory, "nhop-node-linux-arm64", 183)
	return directory
}
