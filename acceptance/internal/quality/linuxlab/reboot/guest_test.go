package reboot

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
	"vpn-script/internal/quality/linuxlab/reboot/guest"
	"vpn-script/internal/quality/linuxlab/reboot/peer"
)

func guestConfig(t *testing.T) contract.Config {
	t.Helper()
	if os.Getenv("NHOP_REBOOT_GUEST") != "owned-vm" {
		t.Skip("dedicated Linux reboot VM only")
	}
	name, err := os.Hostname()
	require(t, err)
	config, err := guest.Require(strings.TrimPrefix(name, "lima-"))
	require(t, err)
	return config
}

func TestRebootPrepare(t *testing.T) {
	config := guestConfig(t)
	require(t, guest.Prepare(context.Background(), config))
}

func TestRebootArm(t *testing.T) {
	config := guestConfig(t)
	if config.Role != "boot-client" {
		t.Fatal("client operation on exit")
	}
	require(t, guest.Arm(context.Background(), config))
}

func TestRebootCanary(t *testing.T) {
	config := guestConfig(t)
	if config.Role != "boot-client" {
		t.Fatal("client operation on exit")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	require(t, guest.Canary(ctx, config))
}

func TestRebootRecovery(t *testing.T) {
	config := guestConfig(t)
	if config.Role != "boot-client" {
		t.Fatal("client operation on exit")
	}
	require(t, guest.Recover(context.Background(), config))
}

func TestRebootPeer(t *testing.T) {
	config := guestConfig(t)
	if config.Role != "boot-exit" {
		t.Fatal("exit operation on client")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	require(t, peer.Run(ctx, config))
}

func TestRebootBaseline(t *testing.T) {
	config := guestConfig(t)
	if config.Role != "boot-client" {
		t.Fatal("client operation on exit")
	}
	values, err := guest.Traffic(context.Background(), config, config.ClientIP)
	require(t, err)
	require(t, guest.Baseline(config))
	t.Logf("restored direct HTTP/UDP origins: %v; 40 NIC-bound canary packets sent", values)
}

func TestRebootClean(t *testing.T) {
	config := guestConfig(t)
	require(t, guest.Clean(context.Background(), config))
}
