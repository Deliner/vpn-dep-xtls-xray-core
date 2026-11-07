package probe

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/client"
	"vpn-script/internal/control/probe/ipcheck"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func preflight(ctx context.Context, cases map[string]ClientCase, binaries map[string]string, endpoint string, run process.Runner) error {
	if _, err := ipcheck.Endpoint(endpoint); err != nil {
		return err
	}
	checked := map[string]bool{}
	for _, value := range cases {
		engine := value.Profile.Engine
		if checked[engine] {
			continue
		}
		if err := client.ValidateBinary(ctx, engine, binaries[engine], run); err != nil {
			return err
		}
		checked[engine] = true
	}
	return nil
}

func Verify(ctx context.Context, directory string, binaries map[string]string, endpoint string, run process.Runner) (Report, error) {
	report := Report{VerifiedAt: time.Now().UTC().Format(time.RFC3339), SSH: map[string]SSHResult{}, Routes: map[string]RouteResult{}}
	lock, err := bundle.Lock(directory)
	if err != nil {
		return report, err
	}
	defer lock.Close()
	manifest, err := bundle.Load(directory)
	if err != nil {
		return report, err
	}
	cases, err := clients(directory)
	if err != nil {
		return report, err
	}
	if err := preflight(ctx, cases, binaries, endpoint, run); err != nil {
		return report, err
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	for _, id := range manifest.Order {
		report.SSH[id] = SSH(ctx, manifest.Nodes[id], run, dialer.DialContext)
	}
	for _, name := range dag.Keys(cases) {
		report.Routes[name] = route(ctx, cases[name], binaries[cases[name].Profile.Engine], endpoint)
	}
	if err := safeio.JSON(filepath.Join(directory, "verification.json"), report); err != nil {
		return report, err
	}
	return report, assessment(report)
}

func assessment(report Report) error {
	for _, result := range report.SSH {
		if !result.Pass {
			return fmt.Errorf("SSH verification failed; see verification.json")
		}
	}
	for _, result := range report.Routes {
		if !result.Pass {
			return fmt.Errorf("route verification failed; see verification.json")
		}
	}
	return nil
}
