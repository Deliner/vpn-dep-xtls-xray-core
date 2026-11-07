package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/platform/safeio"
)

func plan(options options, output io.Writer) error {
	imported, err := profile.Load(options.profile)
	if err != nil {
		return err
	}
	tun, err := network.TUN(imported, options.network)
	if err != nil {
		return err
	}
	guard, err := network.Firewall(imported, options.network)
	if err != nil {
		return err
	}
	if err := os.Mkdir(options.output, 0700); err != nil {
		return fmt.Errorf("plan requires a new private directory: %w", err)
	}
	for name, value := range map[string]any{"profile.json": imported, "options.json": options.network, "native.json": imported.Config, "tun.json": tun} {
		if err := safeio.JSON(filepath.Join(options.output, name), value); err != nil {
			return err
		}
	}
	if err := safeio.Atomic(filepath.Join(options.output, "guard.nft"), []byte(guard), 0600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Prepared Linux TUN, DNS and firewall plan: %s\n", options.output)
	return err
}
