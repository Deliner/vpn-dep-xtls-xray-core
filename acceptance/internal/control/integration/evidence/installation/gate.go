package installation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/node/journal"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

// Gate blocks only this test's restart. Its caller must remove it before recovery.
func Gate(ctx context.Context) error {
	if err := OwnedHost(); err != nil {
		return err
	}
	if err := os.Mkdir(dropin, 0755); err != nil {
		return err
	}
	unit := []byte("[Service]\nExecStartPre=/usr/bin/sleep 120\n")
	if err := safeio.Atomic(gateFile, unit, 0644); err != nil {
		return err
	}
	_, err := systemctl(ctx, "daemon-reload")
	return err
}

func Ungate(ctx context.Context) error {
	if err := OwnedHost(); err != nil {
		return err
	}
	var failures []error
	for _, path := range []string{gateFile, dropin} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err)
		}
	}
	_, err := systemctl(ctx, "daemon-reload")
	return errors.Join(append(failures, err)...)
}

// ObservePending verifies and records an installer running in another process.
func ObservePending(ctx context.Context, previous, output string) (Report, error) {
	if err := OwnedHost(); err != nil {
		return Report{}, err
	}
	pid, err := waitForGate(ctx)
	if err != nil {
		return Report{}, err
	}
	report, err := pending(previous, output)
	if err != nil {
		return report, err
	}
	report.ControlPID, report.SubState = pid, "start-pre"
	return report, nil
}

func systemctl(ctx context.Context, args ...string) ([]byte, error) {
	result, err := process.Run(ctx, 5*time.Second, append([]string{"systemctl"}, args...)...)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("interruption systemctl %v: %v: %s", args, err, result.Output)
	}
	return result.Output, nil
}

func waitForGate(ctx context.Context) (int, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := systemctl(ctx, "show", "-p", "ActiveState", "-p", "SubState", "-p", "ControlPID", service)
		if err != nil {
			return 0, err
		}
		fields := map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			key, value, _ := strings.Cut(line, "=")
			fields[key] = value
		}
		pid, _ := strconv.Atoi(fields["ControlPID"])
		if fields["ActiveState"] == "activating" && fields["SubState"] == "start-pre" && pid > 0 {
			command, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
			if err == nil && bytes.Equal(command, []byte("/usr/bin/sleep\x00120\x00")) {
				return pid, nil
			}
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("installer never reached the restart gate: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func pending(previous, output string) (Report, error) {
	record, err := journal.Load(base)
	if err != nil {
		return Report{}, err
	}
	if record == nil || record.Phase != "pending" || record.Old.Previous != previous || record.Next.Previous == previous {
		return Report{}, fmt.Errorf("restart did not retain the expected pending transaction")
	}
	current, err := filepath.EvalSymlinks(base + "/current")
	if err != nil || current != record.Next.Previous {
		return Report{}, fmt.Errorf("restart gate did not follow release switching: %v", err)
	}
	state, err := os.ReadFile(base + "/state.json")
	if err != nil || !bytes.Equal(state, record.Old.State) {
		return Report{}, fmt.Errorf("restart gate no longer has the previous committed state: %v", err)
	}
	raw, err := safeio.Read(filepath.Join(base, journal.Name), 10_000_000)
	if err != nil {
		return Report{}, err
	}
	path := filepath.Join(output, "interrupted-transaction.json")
	if err := safeio.Atomic(path, raw, 0600); err != nil {
		return Report{}, err
	}
	sha, err := safeio.FileHash(path)
	return Report{PreviousRelease: filepath.Base(previous), NextRelease: filepath.Base(current), JournalSHA256: sha}, err
}
