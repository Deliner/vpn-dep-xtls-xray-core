package fleet

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestAssignmentRejectsOtherFleetOrNode(t *testing.T) {
	for _, trust := range []Trust{{FleetID: "another", NodeID: "entry"}, {FleetID: "owned-fleet", NodeID: "exit"}, {FleetID: "", NodeID: "entry"}} {
		f := newSigningFixture(t)
		trust.PublicKey = f.trust.PublicKey
		if _, _, err := Accept(f.signed(t), trust, Receipt{}, f.now); err == nil {
			t.Fatal("assignment accepted for another identity")
		}
	}
}

func TestAssignmentRejectsRollbackAndSameRevisionEquivocation(t *testing.T) {
	f := newSigningFixture(t)
	f.value.Revision = 7
	_, receipt, err := Accept(f.signed(t), f.trust, Receipt{}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	f.value.Revision = 6
	if _, _, err := Accept(f.signed(t), f.trust, receipt, f.now); err == nil {
		t.Fatal("older revision accepted")
	}
	f.value.Revision = 7
	f.value.ExpiresAt++
	if _, _, err := Accept(f.signed(t), f.trust, receipt, f.now); err == nil {
		t.Fatal("different signed payload accepted at the same revision")
	}
	f.value.Revision++
	if _, next, err := Accept(f.signed(t), f.trust, receipt, f.now); err != nil || next.Revision != 8 {
		t.Fatalf("new revision rejected: %v", err)
	}
}

func TestAssignmentRejectsInvalidPersistedReceipt(t *testing.T) {
	f := newSigningFixture(t)
	for _, receipt := range []Receipt{{Revision: 1}, {PayloadSHA256: strings.Repeat("a", 64)}, {Revision: 1, PayloadSHA256: "bad"}} {
		if _, _, err := Accept(f.signed(t), f.trust, receipt, f.now); err == nil {
			t.Fatalf("invalid receipt accepted: %+v", receipt)
		}
	}
}

func TestAssignmentExpiryAndClockSkew(t *testing.T) {
	f := newSigningFixture(t)
	raw := f.signed(t)
	for _, offset := range []time.Duration{-121 * time.Second, time.Hour, 2 * time.Hour} {
		if _, _, err := Accept(raw, f.trust, Receipt{}, f.now.Add(offset)); err == nil {
			t.Fatalf("expired or future assignment accepted at offset %s", offset)
		}
	}
	for _, offset := range []time.Duration{-120 * time.Second, time.Hour - time.Second} {
		if _, _, err := Accept(raw, f.trust, Receipt{}, f.now.Add(offset)); err != nil {
			t.Fatalf("valid boundary rejected at %s: %v", offset, err)
		}
	}
}

func TestAssignmentRejectsAmbiguousSignedJSON(t *testing.T) {
	f := newSigningFixture(t)
	body := encodeJSON(t, f.value)
	for name, payload := range map[string][]byte{
		"duplicate-key":  bytes.Replace(body, []byte(`"version":1`), []byte(`"version":2,"version":1`), 1),
		"case-alias":     bytes.Replace(body, []byte(`"version"`), []byte(`"VERSION"`), 1),
		"unknown-field":  append([]byte(`{"unreviewed":true,`), body[1:]...),
		"trailing-value": append(bytes.Clone(body), []byte(`{}`)...),
		"null":           []byte(`null`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Accept(f.signedBody(t, payload), f.trust, Receipt{}, f.now); err == nil {
				t.Fatal("ambiguous signed payload accepted")
			}
		})
	}
}
