package evidence

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func Capture(ctx context.Context, inputs map[string]Input) (map[string]Runtime, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no runtime identities requested")
	}
	runtimes := make(map[string]Runtime, len(inputs))
	var failures []error
	for _, name := range dag.Keys(inputs) {
		observed, err := inspect(ctx, inputs[name])
		if err != nil {
			observed.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s identity: %w", name, err))
		}
		runtimes[name] = observed
	}
	return runtimes, errors.Join(failures...)
}

func inspect(ctx context.Context, input Input) (Runtime, error) {
	observed := Runtime{ExpectedVersion: input.ExpectedVersion}
	if input.Path == "" {
		return observed, fmt.Errorf("runtime binary path is empty")
	}
	path, err := filepath.Abs(input.Path)
	if err != nil {
		return observed, err
	}
	observed.Path = path
	observed.SHA256, err = safeio.FileHash(path)
	if err != nil {
		return observed, err
	}
	result, err := process.Run(ctx, 10*time.Second, path, "version")
	observed.VersionOutput = strings.TrimSpace(string(result.Output))
	if err != nil {
		return observed, fmt.Errorf("runtime version command failed: %w", err)
	}
	if len(result.Output) >= 65536 || observed.VersionOutput == "" {
		return observed, fmt.Errorf("runtime version output is empty or exceeds limit")
	}
	if err := unchanged(observed); err != nil {
		return observed, err
	}
	return observed, nil
}

// Verify closes the evidence interval after all runtime processes have stopped.
// It detects changed inputs; it is not an attestation against a hostile filesystem.
func Verify(runtimes map[string]Runtime) error {
	if len(runtimes) == 0 {
		return fmt.Errorf("no captured runtime identities")
	}
	var failures []error
	for _, name := range dag.Keys(runtimes) {
		observed := runtimes[name]
		if observed.Error != "" {
			failures = append(failures, fmt.Errorf("%s identity capture failed: %s", name, observed.Error))
			continue
		}
		if err := unchanged(observed); err != nil {
			observed.VerifiedUnchanged, observed.Error = false, err.Error()
			failures = append(failures, fmt.Errorf("%s identity: %w", name, err))
		} else {
			observed.VerifiedUnchanged = true
		}
		runtimes[name] = observed
	}
	return errors.Join(failures...)
}

func unchanged(observed Runtime) error {
	actual, err := safeio.FileHash(observed.Path)
	if err != nil {
		return err
	}
	if actual != observed.SHA256 {
		return fmt.Errorf("runtime binary changed during the test")
	}
	return nil
}
