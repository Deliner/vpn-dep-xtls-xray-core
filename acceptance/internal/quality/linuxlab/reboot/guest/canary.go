package guest

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Canary(ctx context.Context, config contract.Config) error {
	phase, err := Phase(config)
	if err != nil {
		return err
	}
	if err := corruptGuard(ctx, phase); err != nil {
		return err
	}
	boot, err := BootID()
	if err != nil {
		return err
	}
	value := contract.Canary{BootID: boot, Phase: phase.Name, Started: time.Now().UTC().Format(time.RFC3339Nano)}
	path := contract.Root + "/canary-" + phase.Name + ".json"
	if err := safeio.JSON(path, value); err != nil {
		return err
	}
	canaryAttempt(config, phase.Name, &value)
	if err := notifyReady(); err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			value.Finished = time.Now().UTC().Format(time.RFC3339Nano)
			return safeio.JSON(path, value)
		case <-ticker.C:
			canaryAttempt(config, phase.Name, &value)
			if value.Attempts%40 == 0 {
				if err := safeio.JSON(path, value); err != nil {
					return err
				}
			}
		}
	}
}

func canaryAttempt(config contract.Config, phase string, value *contract.Canary) {
	value.Attempts++
	if sendCanary(config, phase) == nil {
		value.Accepted++
	}
}

func corruptGuard(ctx context.Context, phase contract.Phase) error {
	if phase.Name != "guard-failure" {
		return nil
	}
	if _, err := Client(ctx, "service-prepare"); err != nil {
		return err
	}
	if err := safeio.CheckAccess("/run/nhop-client", os.ModeDir|0711, 0, 0); err != nil {
		return err
	}
	return os.Chmod("/run/nhop-client", 0755)
}

func notifyReady() error {
	path := os.Getenv("NOTIFY_SOCKET")
	if path == "" {
		return fmt.Errorf("canary requires a notifying systemd service")
	}
	connection, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer connection.Close()
	_, err = connection.Write([]byte("READY=1\nSTATUS=Owned NIC canary running"))
	return err
}

func sendCanary(config contract.Config, phase string) error {
	dialer := net.Dialer{Timeout: 100 * time.Millisecond, Control: bindInterface}
	connection, err := dialer.Dial("udp4", net.JoinHostPort(config.PeerIP, "18082"))
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return err
	}
	_, err = connection.Write([]byte(config.Token + ":" + phase))
	return err
}

func Baseline(config contract.Config) error {
	for range 40 {
		if err := sendCanary(config, "baseline"); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
