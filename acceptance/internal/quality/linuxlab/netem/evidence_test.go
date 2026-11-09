package netem

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceInventoryRejectsReplacedMissingUnlistedAndLinkedFiles(t *testing.T) {
	for _, damage := range []string{"valid", "changed", "missing", "unlisted", "link", "duplicate", "escape"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			data := []byte("complete evidence\n")
			writeFixture(t, filepath.Join(root, "record"), data)
			manifest := fmt.Sprintf("%x  ./record\n", sha256.Sum256(data))
			switch damage {
			case "changed":
				writeFixture(t, filepath.Join(root, "record"), []byte("changed"))
			case "missing":
				manifest += fmt.Sprintf("%x  ./missing\n", sha256.Sum256(nil))
			case "unlisted":
				writeFixture(t, filepath.Join(root, "extra"), data)
			case "link":
				if err := os.Symlink("record", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				manifest += manifest
			case "escape":
				manifest = fmt.Sprintf("%x  ../record\n", sha256.Sum256(data))
			}
			writeFixture(t, filepath.Join(root, "evidence.sha256"), []byte(manifest))
			if _, err := completeInventory(root, "evidence.sha256"); (err == nil) != (damage == "valid") {
				t.Fatalf("damaged evidence accepted: %s: %v", damage, err)
			}
		})
	}
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRequiresWholePacketsAndAtLeastOneRecord(t *testing.T) {
	valid := make([]byte, 24+16+14)
	binary.LittleEndian.PutUint32(valid, 0xa1b2c3d4)
	binary.LittleEndian.PutUint32(valid[20:], 1)
	binary.LittleEndian.PutUint32(valid[32:], 14)
	binary.LittleEndian.PutUint32(valid[36:], 14)
	for _, damage := range []string{"valid", "header", "empty", "truncated-header", "truncated-payload", "partial-packet"} {
		value := bytes.Clone(valid)
		switch damage {
		case "header":
			value[0] = 0
		case "empty":
			value = value[:24]
		case "truncated-header":
			value = value[:32]
		case "truncated-payload":
			value = value[:len(value)-1]
		case "partial-packet":
			binary.LittleEndian.PutUint32(value[36:], 15)
		}
		count, err := pcapRecords(bytes.NewReader(value))
		if (err == nil && count == 1) != (damage == "valid") {
			t.Fatalf("invalid capture accepted: %s: %d %v", damage, count, err)
		}
	}
}

func TestComparisonRejectsChangesBeyondCongestion(t *testing.T) {
	first := experiment{inputs: map[string]string{"congestion": "default", "network.test": "same", "bin/xray": "same"}}
	for _, test := range []struct {
		inputs map[string]string
		reason string
	}{
		{map[string]string{"congestion": "default", "network.test": "same", "bin/xray": "same"}, "no distinct control"},
		{map[string]string{"congestion": "reno", "network.test": "changed", "bin/xray": "same"}, "changed network.test"},
		{map[string]string{"congestion": "reno", "network.test": "same"}, "input sets differ"},
	} {
		err := compareInputs(first, experiment{inputs: test.inputs})
		if err == nil || !strings.Contains(err.Error(), test.reason) {
			t.Fatalf("uncontrolled comparison accepted or rejected for the wrong reason: %v", err)
		}
	}
}
