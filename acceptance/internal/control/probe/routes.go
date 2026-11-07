package probe

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"vpn-script/internal/control/probe/ipcheck"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func clients(directory string) (map[string]ClientCase, error) {
	var profiles model.Profiles
	if err := safeio.LoadJSON(filepath.Join(directory, "client-profiles.json"), &profiles); err != nil {
		return nil, err
	}
	if profiles.SchemaVersion != 1 || len(profiles.Routes) == 0 {
		return nil, fmt.Errorf("invalid client profiles schema")
	}
	result := map[string]ClientCase{}
	for name, profile := range profiles.Routes {
		if validate.Name(name) != nil {
			return nil, fmt.Errorf("invalid route name")
		}
		config, engine, err := loadClient(filepath.Join(directory, "clients", name+".json"), profile)
		if err != nil {
			return nil, err
		}
		profile.Engine = engine
		result[name] = ClientCase{Profile: profile, Config: config}
	}
	return result, nil
}

func routeIP(ctx context.Context, binary, endpoint string, config model.Object) (string, error) {
	directory, err := os.MkdirTemp("", "nhop-probe-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	local, err := localClient(ctx, binary, directory, config)
	if err != nil {
		return "", err
	}
	defer local.stop()
	client := ipcheck.NewClient(local.port)
	defer client.CloseIdleConnections()
	actual, err := ipcheck.Query(local.ctx, client, endpoint)
	if cause := context.Cause(local.ctx); err != nil || cause != nil {
		return "", errors.Join(err, cause)
	}
	return actual, nil
}

func route(ctx context.Context, value ClientCase, binary, endpoint string) RouteResult {
	result := RouteResult{Expected: value.Profile.ExpectedExitIP, Scope: "HTTPS via SOCKS; UDP and OS DNS not tested"}
	actual, err := routeIP(ctx, binary, endpoint, value.Config)
	if err != nil {
		result.Error = "probe failed; no exit IP confirmed"
		return result
	}
	result.Actual, result.Pass = actual, sameExitIP(result.Expected, actual)
	return result
}

func sameExitIP(expected, actual string) bool {
	want, wantErr := netip.ParseAddr(expected)
	got, gotErr := netip.ParseAddr(actual)
	return wantErr == nil && gotErr == nil && want.Zone() == "" && got.Zone() == "" && want.Unmap() == got.Unmap()
}
