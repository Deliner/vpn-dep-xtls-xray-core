package boot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/control/client/systemd/install"
)

type fakeManager struct {
	t             *testing.T
	transaction   transaction
	calls         []string
	failReload    bool
	guardState    string
	foreign       string
	secondActive  bool
	overrideUnit  string
	overrideField string
}

func fixture(t *testing.T) (transaction, *fakeManager) {
	t.Helper()
	root := t.TempDir()
	tr := transaction{units: root, state: filepath.Join(root, "boot.json"), uid: os.Geteuid(), gid: os.Getegid()}
	tr.snapshot = &install.Snapshot{OwnerToken: strings.Repeat("a", 32), Receipt: install.Receipt{
		Generation: strings.Repeat("b", 64), Unit: filepath.Join(root, install.Service),
		Directory: "/var/lib/nhop-client/releases/" + strings.Repeat("b", 64)}}
	m := &fakeManager{t: t, transaction: tr, guardState: "inactive"}
	tr.command = m.run
	return tr, m
}

func (m *fakeManager) run(ctx context.Context, args ...string) ([]byte, error) {
	m.calls = append(m.calls, strings.Join(args, " "))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch args[0] {
	case "daemon-reload":
		if m.failReload {
			m.failReload = false
			return nil, errors.New("injected reload failure")
		}
		return nil, nil
	case "stop", "reset-failed":
		if len(args) != 3 || args[1] != "--" || args[2] != systemd.BootService || m.foreign != "" {
			m.t.Fatal("unexpected service mutation", args)
		}
		m.guardState = "inactive"
		return nil, nil
	case "show":
		return m.show(args), nil
	default:
		m.t.Fatal("boot registration attempted activation or network mutation", args)
		return nil, nil
	}
}

func (m *fakeManager) show(args []string) []byte {
	m.t.Helper()
	if len(args) != 4 || args[2] != "--" || !strings.HasPrefix(args[1], "--property=") {
		m.t.Fatal("unexpected manager query", args)
	}
	value := m.properties(args[3])
	if args[3] == m.overrideUnit {
		value[m.overrideField] = ""
	}
	var lines []string
	for _, field := range strings.Split(strings.TrimPrefix(args[1], "--property="), ",") {
		lines = append(lines, field+"="+value[field])
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func (m *fakeManager) properties(unit string) map[string]string {
	v := map[string]string{"LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": filepath.Join(m.transaction.units, unit), "UnitFileState": "static"}
	switch unit {
	case install.Service:
		if m.exists("multi-user.target.wants", install.Service) {
			v["UnitFileState"] = "enabled"
		}
	case systemd.BootService:
		if !m.exists(unit) {
			v["LoadState"], v["FragmentPath"], v["UnitFileState"] = "not-found", "", ""
		} else {
			v["ActiveState"] = m.guardState
			if m.guardState == "active" {
				v["SubState"] = "exited"
			}
			v["RequiredBy"] = m.foreign
		}
	case "systemd-networkd.service", "NetworkManager.service":
		if unit == "systemd-networkd.service" || m.secondActive {
			v["ActiveState"], v["SubState"] = "active", "running"
		}
	default:
		m.t.Fatal("unexpected service query", unit)
	}
	if m.exists(unit+".d", dropIn) {
		v["DropInPaths"] = filepath.Join(m.transaction.units, unit+".d", dropIn)
		v["Requires"], v["After"] = systemd.BootService, systemd.BootService
	}
	return v
}

func (m *fakeManager) exists(parts ...string) bool {
	_, err := os.Lstat(filepath.Join(append([]string{m.transaction.units}, parts...)...))
	return err == nil
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
