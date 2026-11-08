package boot

import (
	"context"
	"os"
	"strings"
	"testing"

	"vpn-script/internal/control/client/systemd"
)

func TestEnableIsDurableIdempotentAndDoesNotStartServices(t *testing.T) {
	tr, m := fixture(t)
	first, err := tr.enable(t.Context(), "")
	require(t, err)
	value, found, err := tr.read()
	require(t, err)
	if !found || value.Phase != "enabled" || !first.Enabled || first.NetworkManager != "systemd-networkd.service" {
		t.Fatal("completed boot registration missing", value, first)
	}
	before, err := os.ReadFile(tr.state)
	require(t, err)
	second, err := tr.enable(t.Context(), first.NetworkManager)
	require(t, err)
	after, err := os.ReadFile(tr.state)
	require(t, err)
	if first != second || string(before) != string(after) {
		t.Fatal("repeated enable changed boot identity")
	}
	for _, call := range m.calls {
		if !strings.HasPrefix(call, "show ") && call != "daemon-reload" {
			t.Fatal("enable activated or stopped a service", call)
		}
	}
	_, err = tr.disable(t.Context())
	require(t, err)
	require(t, tr.unregistered(t.Context()))
}

func TestInterruptedEnableRecoversEveryPublicationBoundary(t *testing.T) {
	for boundary := 0; boundary <= 5; boundary++ {
		t.Run(string(rune('0'+boundary)), func(t *testing.T) {
			tr, _ := fixture(t)
			value, err := tr.begin(t.Context(), "")
			require(t, err)
			files, err := tr.artifacts(value)
			require(t, err)
			for _, item := range files[:boundary] {
				require(t, tr.create(item))
			}
			observed, found, err := tr.read()
			require(t, err)
			if !found || observed != value || observed.Phase != "enabling" {
				t.Fatal("publication lost its durable intent")
			}
			_, err = tr.enable(t.Context(), "")
			require(t, err)
			require(t, tr.enabled(t.Context(), value))
		})
	}
}

func TestReloadFailureRetainsRecoverableIntent(t *testing.T) {
	tr, m := fixture(t)
	m.failReload = true
	if _, err := tr.enable(t.Context(), ""); err == nil {
		t.Fatal("reload failure was hidden")
	}
	value, found, err := tr.read()
	require(t, err)
	if !found || value.Phase != "enabling" {
		t.Fatal("failed reload lost the intended registration")
	}
	_, err = tr.enable(t.Context(), "")
	require(t, err)
	m.guardState, m.failReload = "active", true
	if _, err := tr.disable(t.Context()); err == nil {
		t.Fatal("disable reload failure was hidden")
	}
	if _, err := tr.enable(t.Context(), ""); err == nil {
		t.Fatal("unfinished disable silently reverted to enable")
	}
	value, found, err = tr.read()
	require(t, err)
	if !found || value.Phase != "disabling" {
		t.Fatal("disable intent was lost")
	}
	_, err = tr.disable(t.Context())
	require(t, err)
	if m.guardState != "inactive" {
		t.Fatal("unreferenced oneshot service was not deactivated")
	}
}

func TestForeignReverseDependencyPreventsGuardStop(t *testing.T) {
	tr, m := fixture(t)
	_, err := tr.enable(t.Context(), "")
	require(t, err)
	m.guardState, m.foreign = "active", "unrelated.service"
	if _, err := tr.disable(t.Context()); err == nil {
		t.Fatal("boot guard with a foreign dependent was stopped")
	}
	for _, call := range m.calls {
		if call == "stop -- "+systemd.BootService {
			t.Fatal("stop propagated to an unrelated service")
		}
	}
	m.foreign = ""
	_, err = tr.disable(t.Context())
	require(t, err)
}

func TestCancelledOrAmbiguousEnableDoesNotCreateIntent(t *testing.T) {
	for _, scenario := range []string{"cancel", "ambiguous", "mismatch", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			tr, m := fixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			requested := ""
			switch scenario {
			case "cancel":
				cancel()
			case "ambiguous":
				m.secondActive = true
			case "mismatch":
				requested = "NetworkManager.service"
			case "unsupported":
				requested = "networking.service"
			}
			if _, err := tr.enable(ctx, requested); err == nil {
				t.Fatal("unsupported operation succeeded")
			}
			if _, err := os.Lstat(tr.state); !os.IsNotExist(err) {
				t.Fatal("refused operation created boot state", err)
			}
			require(t, tr.noArtifacts())
		})
	}
}

func TestInactiveAlternativeManagerMustRequireProtection(t *testing.T) {
	tr, m := fixture(t)
	_, err := tr.enable(t.Context(), "")
	require(t, err)
	value := m.properties("NetworkManager.service")
	if value["ActiveState"] != "inactive" || !requiresGuard(value) {
		t.Fatal("inactive alternative manager was left unprotected", value)
	}
	m.overrideUnit, m.overrideField = "NetworkManager.service", "Requires"
	if _, err := tr.enable(t.Context(), ""); err == nil {
		t.Fatal("alternative manager missing protection was accepted")
	}
}

func TestGuardMustBeSettledBeforeEnableCompletes(t *testing.T) {
	for _, state := range []string{"activating", "deactivating", "failed"} {
		t.Run(state, func(t *testing.T) {
			tr, m := fixture(t)
			m.guardState = state
			if _, err := tr.enable(t.Context(), ""); err == nil {
				t.Fatal("unsafe guard state accepted", state)
			}
			value, found, err := tr.read()
			require(t, err)
			if !found || value.Phase != "enabling" {
				t.Fatal("unsafe guard state discarded recovery intent")
			}
		})
	}
}
