package fleet

import (
	"bytes"
	"testing"
	"time"
)

func TestAssignmentConstructionCopiesAndBoundsManifest(t *testing.T) {
	f := newSigningFixture(t)
	input := bytes.Clone(f.value.Manifest)
	value, err := NewAssignment("owned-fleet", "entry", 1, input, f.now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = '!'
	if !bytes.Equal(value.Manifest, f.value.Manifest) {
		t.Fatal("assignment aliases caller-owned mutable manifest")
	}
	for _, raw := range [][]byte{nil, []byte(`not JSON`), bytes.Repeat([]byte{' '}, MaxManifestSize+1)} {
		if _, err := NewAssignment("owned-fleet", "entry", 1, raw, f.now, time.Minute); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	for _, validity := range []time.Duration{0, time.Nanosecond, (MaxValiditySeconds + 1) * time.Second} {
		if _, err := NewAssignment("owned-fleet", "entry", 1, f.value.Manifest, f.now, validity); err == nil {
			t.Fatal("invalid validity accepted")
		}
	}
}

func TestAssignmentPolicyRejectsValidlySignedInvalidClaims(t *testing.T) {
	for name, change := range map[string]func(*Assignment){
		"checksum": func(a *Assignment) {
			a.Manifest = bytes.Replace(a.Manifest, []byte("entry"), []byte("other"), 1)
		},
		"wrong-schema":   func(a *Assignment) { a.Version = 2 },
		"zero-revision":  func(a *Assignment) { a.Revision = 0 },
		"bad-node":       func(a *Assignment) { a.NodeID = "../entry" },
		"invalid-time":   func(a *Assignment) { a.IssuedAt = 0 },
		"empty-validity": func(a *Assignment) { a.ExpiresAt = a.IssuedAt },
		"long-validity":  func(a *Assignment) { a.ExpiresAt = a.IssuedAt + MaxValiditySeconds + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			f := newSigningFixture(t)
			change(&f.value)
			if _, _, err := Accept(f.signedBody(t, encodeJSON(t, f.value)), f.trust, Receipt{}, f.now); err == nil {
				t.Fatal("invalid claims accepted despite authenticated signer")
			}
		})
	}
}

func TestEnvelopeSizeLimit(t *testing.T) {
	f := newSigningFixture(t)
	if _, _, err := Accept(bytes.Repeat([]byte{' '}, MaxEnvelopeSize+1), f.trust, Receipt{}, f.now); err == nil {
		t.Fatal("oversized envelope accepted")
	}
}
