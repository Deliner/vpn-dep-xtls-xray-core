package bootstrap

import (
	"testing"

	"vpn-script/internal/control/client/guard"
)

func TestInterruptedAcquisitionKeepsIdentityAndNeverDuplicatesTable(t *testing.T) {
	for _, boundary := range []string{"before-create", "after-create"} {
		t.Run(boundary, func(t *testing.T) {
			s, k := fixture(t, boundary)
			if s.activate(t.Context()) == nil {
				t.Fatal("injected interruption was hidden")
			}
			value, found, err := s.read()
			require(t, err)
			if !found || value.Guard.Token == "" || value.Guard.Handle != 0 {
				t.Fatal("pending intent was lost", value)
			}
			require(t, s.activate(t.Context()))
			status, err := s.inspect(t.Context())
			require(t, err)
			if k.creations != 1 || status.Phase != "protected" || status.Guard.Token != value.Guard.Token {
				t.Fatal("acquisition recovery changed identity or duplicated the table", status, k.creations)
			}
		})
	}
}

func TestReleaseIntentSurvivesFailureBeforeAndAfterKernelDeletion(t *testing.T) {
	for _, boundary := range []string{"before-delete", "after-delete"} {
		t.Run(boundary, func(t *testing.T) {
			s, k := fixture(t, boundary)
			require(t, s.activate(t.Context()))
			initial := k.current
			if s.release(t.Context(), false) == nil {
				t.Fatal("injected deletion failure was hidden")
			}
			value, found, err := s.read()
			require(t, err)
			if !found || !value.Releasing || value.Guard != initial {
				t.Fatal("release intent missing", value)
			}
			require(t, s.release(t.Context(), false))
			status, err := s.inspect(t.Context())
			require(t, err)
			if status.Phase != "released" || k.current != (guard.Lease{}) {
				t.Fatal("release did not recover", status)
			}
			if s.activate(t.Context()) == nil {
				t.Fatal("handoff implicitly rearmed boot protection")
			}
		})
	}
}

func TestUnexpectedLossOrReplacementRemainsAnError(t *testing.T) {
	for _, replacement := range []string{"missing", "token", "handle"} {
		t.Run(replacement, func(t *testing.T) {
			s, k := fixture(t, "")
			require(t, s.activate(t.Context()))
			switch replacement {
			case "missing":
				k.current = guard.Lease{}
			case "token":
				k.current.Token = guard.NewIntent().Token
			case "handle":
				k.current.Handle++
			}
			before := k.current
			if s.activate(t.Context()) == nil {
				t.Fatal("unknown table adopted during activation")
			}
			if s.release(t.Context(), true) == nil {
				t.Fatal("unknown table accepted during explicit stop")
			}
			if _, err := s.inspect(t.Context()); err == nil {
				t.Fatal("unknown table reported as healthy")
			}
			if k.current != before {
				t.Fatal("unowned table was mutated")
			}
		})
	}
}

func TestExplicitStopAllowsFreshActivationAndRejectsUnrecordedTables(t *testing.T) {
	s, k := fixture(t, "")
	k.current = guard.Lease{Token: guard.NewIntent().Token, Handle: 91}
	if s.activate(t.Context()) == nil {
		t.Fatal("unrecorded table adopted")
	}
	k.current = guard.Lease{}
	require(t, s.activate(t.Context()))
	initial := k.current
	require(t, s.release(t.Context(), true))
	_, found, err := s.read()
	require(t, err)
	if found {
		t.Fatal("explicit stop retained its acquisition journal")
	}
	require(t, s.activate(t.Context()))
	if k.current.Token == initial.Token || k.creations != 2 {
		t.Fatal("fresh explicit activation reused identity")
	}
}
