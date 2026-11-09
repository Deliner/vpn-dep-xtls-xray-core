package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func TestFailedUpdateRestoresReleaseUnitFirewallAndState(t *testing.T) {
	i, backend, data, next := setup(t, true)
	before, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
	require(t, err)
	data.Firewall, backend.fail = "ufw", "systemctl enable "+service
	if err := i.activate(data, next); err == nil {
		t.Fatal("injected failure was ignored")
	}
	current, err := i.current()
	require(t, err)
	if filepath.Base(current) != strings.Repeat("a", 64) {
		t.Fatal("release pointer not rolled back")
	}
	for path, expected := range map[string]string{i.Paths.Unit: marker + "\nold unit\n", i.Paths.Firewall: marker + "\nold ports\n",
		filepath.Join(i.Paths.Base, "state.json"): string(before)} {
		contents, err := os.ReadFile(path)
		require(t, err)
		if string(contents) != expected {
			t.Fatalf("not restored: %s", path)
		}
	}
	if backend.calls[len(backend.calls)-1] != "systemctl restart "+service {
		t.Fatal("previous active service was not restarted")
	}
}

func TestFirstInstallFailureRemovesPointerAndUnit(t *testing.T) {
	i, backend, data, next := setup(t, false)
	backend.fail = "systemctl restart " + service
	if err := i.activate(data, next); err == nil {
		t.Fatal("injected failure was ignored")
	}
	for _, path := range []string{i.Paths.Unit, filepath.Join(i.Paths.Base, "current"), filepath.Join(i.Paths.Base, "state.json")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed initial deployment left %s", path)
		}
	}
}

func TestCancellationDoesNotCancelRollback(t *testing.T) {
	i, backend, data, next := setup(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	i.Context = ctx
	i.Run = func(ctx context.Context, limit time.Duration, args ...string) (process.Result, error) {
		if strings.Join(args, " ") == "systemctl restart "+service {
			cancel()
		}
		return backend.run(ctx, limit, args...)
	}
	if err := i.activate(data, next); err == nil {
		t.Fatal("cancelled apply unexpectedly succeeded")
	}
	current, err := i.current()
	require(t, err)
	if filepath.Base(current) != strings.Repeat("a", 64) || !strings.Contains(strings.Join(backend.calls, "\n"), "systemctl daemon-reload") {
		t.Fatal("rollback did not use an independent context")
	}
}

func TestSuccessfulCommitIsIdempotentAndDetectsDrift(t *testing.T) {
	i, _, data, next := setup(t, false)
	require(t, i.activate(data, next))
	var state State
	require(t, safeio.LoadJSON(filepath.Join(i.Paths.Base, "state.json"), &state))
	unchanged, err := i.unchanged(&state, data)
	require(t, err)
	if !unchanged {
		t.Fatal("successful release is not recognized as unchanged")
	}
	require(t, os.WriteFile(filepath.Join(next, "config.json"), []byte("changed"), 0640))
	unchanged, err = i.unchanged(&state, data)
	require(t, err)
	if unchanged {
		t.Fatal("configuration drift not detected")
	}
}

func TestForeignPortOwnerIsRejected(t *testing.T) {
	i, backend, data, _ := setup(t, false)
	backend.ports = strings.ReplaceAll(backend.ports, "pid=123", "pid=456")
	if err := i.checkPorts(data); err == nil {
		t.Fatal("foreign port owner accepted")
	}
}

func TestDisjointForeignAddressDoesNotOccupyRequestedSocket(t *testing.T) {
	i, backend, data, _ := setup(t, false)
	data.Listeners[0].Address = "127.0.0.1"
	backend.ports = "tcp LISTEN 0 4096 127.0.0.2:24443 0.0.0.0:* users:((\"other\",pid=456,fd=7))\n"
	if err := i.checkPorts(data); err != nil {
		t.Fatal("disjoint literal address was rejected", err)
	}
}

func TestReadinessRejectsWrongAddressAndUnexpectedPublicExposure(t *testing.T) {
	for _, address := range []string{"127.0.0.2", "0.0.0.0", "[::]"} {
		i, backend, data, _ := setup(t, false)
		data.Listeners[0].Address = "127.0.0.1"
		backend.ports = strings.ReplaceAll(backend.ports, "0.0.0.0:24443", address+":24443")
		if address == "[::]" {
			backend.ports = strings.TrimSpace(backend.ports) + " v6only:0\n"
		}
		sockets, err := i.sockets()
		require(t, err)
		if listenersReady(data.Listeners, sockets, "123") {
			t.Errorf("listener declared ready at unexpected address %s", address)
		}
	}
}

func TestMalformedSocketObservationDoesNotMeanFreePorts(t *testing.T) {
	i, backend, _, _ := setup(t, false)
	backend.ports = "tcp LISTEN 0 4096 invalid-binding 0.0.0.0:*\n"
	if _, err := i.sockets(); err == nil {
		t.Fatal("malformed socket observation was silently discarded")
	}
}

func TestReadinessPreservesTheSocketObservationFailure(t *testing.T) {
	i, backend, data, _ := setup(t, false)
	backend.ports = "unreadable socket data\n"
	if err := i.ready(data); err == nil || !strings.Contains(err.Error(), "invalid socket observation") {
		t.Fatal("readiness lost its observation failure", err)
	}
}

func TestOutputLimitCannotBecomeAnIncompleteSocketInventory(t *testing.T) {
	i, backend, _, _ := setup(t, false)
	backend.ports += strings.Repeat(" ", process.OutputLimit-len(backend.ports))
	if _, err := i.sockets(); err == nil || !strings.Contains(err.Error(), "completeness is unknown") {
		t.Fatal("bounded command output was trusted as complete", err)
	}
}

func TestKernelListenerObservation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux ss and procfs socket observation")
	}
	i, _, _, _ := setup(t, false)
	i.Run = process.Run
	for _, check := range [][2]string{{"tcp4", "127.0.0.1"}, {"udp4", "127.0.0.2"},
		{"tcp6", "::1"}, {"udp6", "::1"}, {"tcp", "0.0.0.0"}, {"udp", "0.0.0.0"},
		{"tcp6", "::"}, {"udp6", "::"}} {
		listener := kernelListener(t, check[0], check[1])
		observed, err := i.sockets()
		require(t, err)
		if !listenersReady([]model.Listener{listener}, observed, strconv.Itoa(os.Getpid())) {
			t.Fatalf("kernel listener not recognized: %+v; %+v", listener, observed)
		}
	}
}

func TestChildProcessSocketsBelongToSupervisor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux procfs ownership")
	}
	command := process.Command(t.Context(), "sleep", "30")
	require(t, command.Start())
	t.Cleanup(func() { _ = process.Stop(command); _ = command.Wait() })
	child, owner := strconv.Itoa(command.Process.Pid), strconv.Itoa(os.Getpid())
	if !ownedPID(child, owner) || !ownedPID(owner, owner) {
		t.Fatal("supervisor does not own its own process and direct child")
	}
	for _, foreign := range []string{strconv.Itoa(os.Getppid()), "0", "-1", "", "../self"} {
		if ownedPID(child, foreign) {
			t.Fatalf("foreign or invalid owner accepted: %s", foreign)
		}
	}
}

func TestMultiEngineCommitDetectsEveryFileDrift(t *testing.T) {
	i, _, data, next := setup(t, false)
	programRelease(t, &data, next)
	require(t, i.activate(data, next))
	var state State
	require(t, safeio.LoadJSON(filepath.Join(i.Paths.Base, "state.json"), &state))
	if len(state.Files) != 7 {
		t.Fatal("incomplete release inventory")
	}
	unchanged, err := i.unchanged(&state, data)
	require(t, err)
	if !unchanged {
		t.Fatal("multi-engine release is not idempotent")
	}
	for name := range state.Files {
		path := filepath.Join(next, name)
		saved, err := os.ReadFile(path)
		require(t, err)
		require(t, os.WriteFile(path, []byte("tampered"), 0640))
		unchanged, err := i.unchanged(&state, data)
		require(t, err)
		if unchanged {
			t.Fatalf("did not detect drift: %s", name)
		}
		require(t, os.WriteFile(path, saved, 0640))
	}
}

func TestFailedMultiEngineCommitRollsBackToLegacyRelease(t *testing.T) {
	i, _, data, next := setup(t, true)
	before, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
	require(t, err)
	programRelease(t, &data, next)
	require(t, os.Remove(filepath.Join(next, "libcronet.so")))
	if err := i.activate(data, next); err == nil {
		t.Fatal("incomplete release committed")
	}
	current, err := i.current()
	require(t, err)
	if filepath.Base(current) != strings.Repeat("a", 64) {
		t.Fatal("release not rolled back")
	}
	unit, err := os.ReadFile(i.Paths.Unit)
	require(t, err)
	if string(unit) != marker+"\nold unit\n" {
		t.Fatal("legacy unit not restored")
	}
	state, err := os.ReadFile(filepath.Join(i.Paths.Base, "state.json"))
	require(t, err)
	if string(state) != string(before) {
		t.Fatal("previous state not restored")
	}
}

func TestNewSupervisorDoesNotReuseOlderCommittedBinary(t *testing.T) {
	i, _, data, next := setup(t, false)
	programRelease(t, &data, next)
	require(t, os.WriteFile(filepath.Join(next, "nhop-node"), []byte("previous supervisor build"), 0755))
	require(t, i.activate(data, next))
	var state State
	require(t, safeio.LoadJSON(filepath.Join(i.Paths.Base, "state.json"), &state))
	unchanged, err := i.unchanged(&state, data)
	require(t, err)
	if unchanged {
		t.Fatal("new installer silently kept the previously committed supervisor executable")
	}
}

func TestManagedPathsRejectUnsafeControlFilesWithoutRepair(t *testing.T) {
	for _, name := range []string{"unit", "ufw", "state.json", "install.lock", "last-error.log", "cache"} {
		t.Run(name, func(t *testing.T) {
			i, _, data, directory := setup(t, false)
			require(t, i.activate(data, directory))
			path := filepath.Join(i.Paths.Base, name)
			if name == "cache" {
				require(t, os.Mkdir(path, 0700))
			} else if name != "unit" && name != "state.json" {
				mode := os.FileMode(0600)
				if name == "ufw" {
					mode = 0644
				}
				require(t, safeio.Atomic(path, []byte("fixture"), mode))
			}
			require(t, i.managedPaths())
			require(t, os.Chmod(path, 0777))
			if err := i.managedPaths(); err == nil || !strings.Contains(err.Error(), "unsafe managed access") {
				t.Fatal("unsafe control path accepted", err)
			}
			info, err := os.Lstat(path)
			require(t, err)
			if info.Mode().Perm() != 0777 {
				t.Fatal("preflight silently repaired an unsafe path")
			}
		})
	}
}
