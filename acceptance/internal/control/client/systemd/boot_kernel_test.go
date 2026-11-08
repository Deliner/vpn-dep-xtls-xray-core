package systemd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelBootOrderingFailsClosed(t *testing.T) {
	f := bootSetup(t)
	f.ping(true)
	if err := os.Chmod(f.state, 0755); err != nil {
		t.Fatal(err)
	}
	f.writeConsumer(false)
	f.control(true, "start", "--", f.consumer)
	if _, err := os.Stat(filepath.Join(f.root, "reached")); err != nil {
		t.Fatal("weak-dependency control did not start after guard failure", err)
	}
	t.Log("control: Wants+After permits dependent activation after protection failure")
	f.control(true, "stop", "--", f.consumer)
	if err := os.Remove(filepath.Join(f.root, "reached")); err != nil {
		t.Fatal(err)
	}
	f.writeConsumer(true)
	f.control(true, "reset-failed", "--", f.guard)
	f.control(false, "start", "--", f.consumer)
	if _, err := os.Stat(filepath.Join(f.root, "reached")); !os.IsNotExist(err) {
		t.Fatal("dependent ran despite failed mandatory protection", err)
	}
	t.Log("required protection: failure blocks dependent activation")
	if err := os.Chmod(f.state, 0711); err != nil {
		t.Fatal(err)
	}
	f.control(true, "reset-failed", "--", f.guard)
	f.control(true, "start", "--", f.consumer)
	verifyBootLifetime(t, f)
}

func verifyBootLifetime(t *testing.T, f *bootFixture) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(f.root, "reached")); err != nil {
		t.Fatal("protected dependent did not start", err)
	}
	data := f.cli("diagnostics")
	if !strings.Contains(string(data), `"phase":"protected"`) && !strings.Contains(string(data), `"phase": "protected"`) {
		t.Fatal("real early policy is not protected", string(data))
	}
	f.ping(false)
	f.control(true, "stop", "--", f.guard)
	f.ping(false)
	f.cli("stop")
	f.ping(true)
	t.Log("native systemd verified: successful protection precedes dependent; unit stop retains firewall; explicit client stop restores connectivity")
}
