package fleet

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

func Sign(value Assignment, key ed25519.PrivateKey, now time.Time) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize || !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])) {
		return nil, fmt.Errorf("invalid Ed25519 signing key")
	}
	if err := validateAssignment(value, now); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	signed := ed25519.Sign(key, preAuthEncoding(PayloadType, payload))
	result := envelope{PayloadType: PayloadType, Payload: base64.StdEncoding.EncodeToString(payload),
		Signatures: []signature{{KeyID: Digest(key[ed25519.SeedSize:]), Value: base64.StdEncoding.EncodeToString(signed)}}}
	return json.Marshal(result)
}

func verifyPayload(raw []byte, key ed25519.PublicKey) ([]byte, error) {
	if len(raw) == 0 || len(raw) > MaxEnvelopeSize || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid envelope size or verification key")
	}
	var value envelope
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("invalid signed envelope JSON")
	}
	if value.PayloadType != PayloadType || len(value.Signatures) == 0 || len(value.Signatures) > 8 {
		return nil, fmt.Errorf("unsupported payload type or signature count")
	}
	payload, err := decodeBase64(value.Payload)
	if err != nil || len(payload) > 16<<20 {
		return nil, fmt.Errorf("invalid signed payload")
	}
	message := preAuthEncoding(value.PayloadType, payload)
	for _, item := range value.Signatures {
		signed, err := decodeBase64(item.Value)
		// keyid is an unauthenticated hint; only the pinned key grants trust.
		if err == nil && ed25519.Verify(key, message, signed) {
			return payload, nil
		}
	}
	return nil, fmt.Errorf("assignment signature verification failed")
}
