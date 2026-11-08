package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestUnsafeManagerStatesRefuseBeforePublication(t *testing.T) {
	cases := []map[string]string{{"ActiveState": "active", "SubState": "running"}, {"ActiveState": "activating"},
		{"Job": "123"}, {"DropInPaths": "/etc/systemd/system/nhop-client.service.d/override.conf"},
		{"LoadState": "masked"}, {"UnitFileState": "enabled"}, {"FragmentPath": "/usr/lib/systemd/system/nhop-client.service"}}
	for _, state := range cases {
		t.Run(strings.Join(mapKeys(state), ","), func(t *testing.T) {
			tr, manager, b := installation(t)
			manager.state = state
			if _, err := tr.apply(context.Background(), b); err == nil {
				t.Fatal("unsafe or overridden manager state was accepted", state)
			}
			if _, err := os.Lstat(tr.store.unit); !os.IsNotExist(err) {
				t.Fatal("rejected install published a unit", err)
			}
			entries, err := os.ReadDir(filepath.Join(tr.store.base, "releases"))
			if err != nil || len(entries) != 0 {
				t.Fatal("rejected install left an unnecessary generation", err)
			}
		})
	}
}

func mapKeys(value map[string]string) []string {
	var result []string
	for key := range value {
		result = append(result, key)
	}
	return result
}

func TestStoreRefusesConcurrentInstallerAndUnownedState(t *testing.T) {
	tr, _, _ := installation(t)
	if extra, err := openStore(tr.store.base, tr.store.unit); err == nil {
		extra.lock.Close()
		t.Fatal("second installer acquired the same store")
	}
	for _, name := range []string{"empty", "writable", "symlink"} {
		t.Run(name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "client")
			var err error
			if name == "symlink" {
				err = os.Symlink(tr.store.base, base)
			} else {
				err = os.Mkdir(base, 0700)
				if name == "writable" && err == nil {
					err = os.Chmod(base, 0777)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if s, err := openStore(base, tr.store.unit); err == nil {
				s.lock.Close()
				t.Fatal("foreign store was adopted")
			}
		})
	}
}

func TestInputsRejectChangedRuntimeWrongCPUAndUnsafeProfile(t *testing.T) {
	for _, problem := range []string{"runtime-hash", "elf", "cpu", "profile", "policy"} {
		t.Run(problem, func(t *testing.T) {
			options := inputFiles(t)
			executable := elfBytes(1)
			switch problem {
			case "runtime-hash":
				if err := safeio.Atomic(filepath.Join(filepath.Dir(options.Runtimes), "xray"), elfBytes(9), 0755); err != nil {
					t.Fatal(err)
				}
			case "elf":
				executable = []byte("#!/bin/sh\n")
			case "cpu":
				executable[18] = 0
			case "profile":
				if err := safeio.Atomic(options.Profile, []byte(`{"schema_version":99}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "policy":
				options.Network.IPv6 = "direct"
			}
			if _, err := loadBundle(options, executable); err == nil {
				t.Fatal("invalid installation input was accepted", problem)
			}
		})
	}
}

func TestDuplicateOrMissingSystemdFieldsAreNotAnInactiveService(t *testing.T) {
	for _, raw := range []string{"", "ActiveState=inactive\n", "ActiveState=inactive\nActiveState=active\n", "invalid"} {
		if _, err := parseState([]byte(raw)); err == nil {
			t.Fatal("incomplete manager response accepted")
		}
	}
}
