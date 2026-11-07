package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func kernelProcess(t *testing.T, directory, label string, args ...string) {
	t.Helper()
	log, err := os.Create(filepath.Join(directory, label+".log"))
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), args[0], args[1:]...)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		process.Stop(command)
		command.Wait()
		log.Close()
		raw, err := os.ReadFile(log.Name())
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s: %s", label, raw)
	})
}

func kernelResolved(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	// The daemon and D-Bus clients may drop privileges inside this isolated guest.
	for _, path := range []string{filepath.Dir(directory), directory} {
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	address := "unix:path=" + filepath.Join(directory, "bus")
	config := "<busconfig><type>system</type><listen>" + address + "</listen><auth>EXTERNAL</auth><policy context=\"default\"><allow user=\"*\"/><allow own=\"*\"/><allow send_destination=\"*\"/><allow receive_sender=\"*\"/></policy></busconfig>"
	path := filepath.Join(directory, "bus.conf")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", address)
	kernelProcess(t, directory, "bus", "dbus-daemon", "--nofork", "--nopidfile", "--config-file="+path)
	waitKernelSocket(t, filepath.Join(directory, "bus"))
	kernelProcess(t, directory, "resolved", "/usr/lib/systemd/systemd-resolved")
	deadline := time.Now().Add(5 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		result, err := process.Run(t.Context(), time.Second, "resolvectl", "dns")
		if err == nil && result.Success {
			return
		}
		last = string(result.Output)
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("private resolved did not acquire its D-Bus name: ", last)
}

func waitKernelSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("private D-Bus socket was not created")
}
