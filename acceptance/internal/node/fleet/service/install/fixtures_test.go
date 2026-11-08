package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	tx         transaction
	root       string
	loaded     bool
	active     bool
	enabled    bool
	reloads    int
	resets     int
	failReload int
	cancel     context.CancelFunc
	override   map[string]string
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	require(t, os.Chmod(root, 0700))
	f := &fixture{root: root, override: map[string]string{}}
	s, err := openStore(filepath.Join(root, "service"), filepath.Join(root, "agent.service"), true)
	require(t, err)
	f.tx = transaction{store: s, manager: f.manage}
	t.Cleanup(func() { require(t, f.tx.store.lock.Close()) })
	return f
}

func (f *fixture) apply(t *testing.T, contents string) Receipt {
	t.Helper()
	value, err := f.tx.apply(context.Background(), Options{State: "/var/lib/fleet-node", Interval: time.Second}, []byte(contents))
	require(t, err)
	return value
}

func (f *fixture) manage(ctx context.Context, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch args[0] {
	case "show":
		return f.state(), nil
	case "daemon-reload":
		f.reloads++
		if f.failReload > 0 {
			f.failReload--
			if f.cancel != nil {
				f.cancel()
			}
			return nil, fmt.Errorf("simulated systemd reload failure")
		}
		_, err := os.Stat(f.tx.store.unit)
		f.loaded = err == nil
	case "enable":
		f.active, f.enabled = true, true
	case "disable":
		f.active, f.enabled = false, false
	case "reset-failed":
		f.resets++
		delete(f.override, "ActiveState")
		delete(f.override, "SubState")
	default:
		return nil, fmt.Errorf("unexpected manager operation")
	}
	return nil, nil
}

func (f *fixture) state() []byte {
	value := map[string]string{"LoadState": "not-found", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "", "DropInPaths": "", "UnitFileState": "", "Job": ""}
	if f.loaded {
		value["LoadState"], value["FragmentPath"], value["UnitFileState"] = "loaded", f.tx.store.unit, "disabled"
	}
	if f.active {
		value["ActiveState"], value["SubState"] = "active", "running"
	}
	if f.enabled {
		value["UnitFileState"] = "enabled"
	}
	for key, item := range f.override {
		value[key] = item
	}
	var result strings.Builder
	for _, key := range strings.Split(properties, ",") {
		fmt.Fprintf(&result, "%s=%s\n", key, value[key])
	}
	return []byte(result.String())
}
