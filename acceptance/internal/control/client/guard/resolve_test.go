package guard

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/platform/process"
)

type resolutionCase struct {
	name, identity, tables, policy string
	lease, want                    Lease
	ok                             bool
}

func TestResolveRequiresRecordedIntentOrExactCompletedLease(t *testing.T) {
	pending := NewIntent()
	complete := Lease{Token: pending.Token, Handle: 17}
	identity := fmt.Sprintf(`{"nftables":[{"table":{"family":"inet","name":"nhop_client","handle":17,"comment":%q}}]}`, pending.Token)
	present := `{"nftables":[{"table":{"family":"inet","name":"nhop_client"}}]}`
	absent := `{"nftables":[]}`
	for _, value := range []resolutionCase{
		{"pending-installed", identity, present, protectedChains, pending, complete, true},
		{"pending-not-installed", "", absent, "", pending, Lease{}, true},
		{"completed", identity, present, protectedChains, complete, complete, true},
		{"completed-missing", "", absent, "", complete, Lease{}, false},
		{"completed-replaced", strings.Replace(identity, `"handle":17`, `"handle":18`, 1), present, protectedChains, complete, Lease{}, false},
		{"foreign", strings.Replace(identity, pending.Token, NewIntent().Token, 1), present, protectedChains, pending, Lease{}, false},
		{"no-forward-drop", identity, present, strings.Replace(protectedChains, `"hook":"forward"`, `"hook":"input"`, 1), pending, Lease{}, false},
		{"unknown-identity", `{}`, present, protectedChains, pending, Lease{}, false},
		{"unknown-absence", "", `{}`, "", pending, Lease{}, false},
		{"empty-token", identity, present, protectedChains, Lease{}, Lease{}, false},
		{"invalid-token", identity, present, protectedChains, Lease{Token: "foreign"}, Lease{}, false},
	} {
		t.Run(value.name, func(t *testing.T) { checkResolution(t, value) })
	}
}

func checkResolution(t *testing.T, value resolutionCase) {
	t.Helper()
	m := &Manager{run: func(_ context.Context, _ time.Duration, args ...string) (process.Result, error) {
		var output string
		switch strings.Join(args, " ") {
		case "nft --json --handle list table inet nhop_client":
			output = value.identity
		case "nft --json list tables":
			output = value.tables
		case "nft --json list table inet nhop_client":
			output = value.policy
		default:
			t.Fatal("intent resolution attempted mutation or an unknown command", args)
		}
		if output == "" {
			return process.Result{}, fmt.Errorf("table unavailable")
		}
		return process.Result{Success: true, Output: []byte(output)}, nil
	}}
	actual, err := m.Resolve(t.Context(), value.lease)
	if (err == nil) != value.ok || value.ok && actual != value.want {
		t.Fatalf("intent resolution mismatch: %+v, %v", actual, err)
	}
}

func TestAcquireReservedRejectsMissingOrCompletedIntentBeforeCommands(t *testing.T) {
	m := &Manager{run: func(_ context.Context, _ time.Duration, args ...string) (process.Result, error) {
		t.Fatal("invalid intention attempted a command", args)
		return process.Result{}, nil
	}}
	for _, lease := range []Lease{{}, {Token: "foreign"}, {Token: NewIntent().Token, Handle: 1}} {
		if _, err := m.AcquireReserved(t.Context(), profile.Imported{}, network.Options{}, lease); err == nil {
			t.Fatal("invalid intention accepted", lease)
		}
	}
}

const protectedChains = `{"nftables":[
 {"chain":{"family":"inet","table":"nhop_client","name":"output","type":"filter","hook":"output","prio":50,"policy":"drop"}},
 {"chain":{"family":"inet","table":"nhop_client","name":"forward","type":"filter","hook":"forward","prio":50,"policy":"drop"}}
]}`
