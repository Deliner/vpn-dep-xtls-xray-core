package packets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPacketProbeRejectsUnreviewedSourceAndOccupiedHelpers(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "transport", "internet", "masque", "connectip")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "conn.go")
	original := []byte("package connectip\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectConnectIP(root); err == nil {
		t.Fatal("accepted unreviewed CONNECT-IP source")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(original) {
		t.Fatal("rejected overlay changed source")
	}
	occupied := filepath.Join(directory, "nhop_connectip_header.go")
	if err := os.Symlink(path, occupied); err != nil {
		t.Fatal(err)
	}
	if _, err := preparePacketHelpers(directory, "connectip", packetBindingControl); err == nil {
		t.Fatal("accepted an occupied overlay destination")
	}
}

func TestDeviceProbeRejectsUnreviewedSource(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "proxy", "wireguard")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "netstack.go")
	original := []byte("package wireguard\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectDevice(root); err == nil {
		t.Fatal("accepted unreviewed device source")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(original) {
		t.Fatal("rejected device overlay changed source")
	}
	if _, err := preparePacketHelpers(directory, "unreviewed", packetBindingControl); err == nil {
		t.Fatal("accepted unreviewed diagnostic target package")
	}
}
