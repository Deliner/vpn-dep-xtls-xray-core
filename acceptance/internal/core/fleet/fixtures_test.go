package fleet

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

type signingFixture struct {
	key   ed25519.PrivateKey
	trust Trust
	value Assignment
	now   time.Time
}

func newSigningFixture(t *testing.T) signingFixture {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{41}, ed25519.SeedSize))
	now := time.Unix(1_800_000_000, 0)
	value, err := NewAssignment("owned-fleet", "entry", 1, []byte("{\n  \"node_id\": \"entry\"\n}\n"), now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signingFixture{key: key, trust: Trust{FleetID: "owned-fleet", NodeID: "entry", PublicKey: key.Public().(ed25519.PublicKey)}, value: value, now: now}
}

func encodeJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f signingFixture) signedBody(t *testing.T, payload []byte) []byte {
	t.Helper()
	signed := ed25519.Sign(f.key, preAuthEncoding(PayloadType, payload))
	return encodeJSON(t, envelope{PayloadType: PayloadType, Payload: base64.StdEncoding.EncodeToString(payload),
		Signatures: []signature{{Value: base64.StdEncoding.EncodeToString(signed)}}})
}

func (f signingFixture) signed(t *testing.T) []byte {
	t.Helper()
	raw, err := Sign(f.value, f.key, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
