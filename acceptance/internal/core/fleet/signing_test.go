package fleet

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDSSEPreAuthenticationVectors(t *testing.T) {
	for _, vector := range []struct{ kind, body, encoded string }{
		{"http://example.com/HelloWorld", "hello world", "DSSEv1 29 http://example.com/HelloWorld 11 hello world"},
		{"", "", "DSSEv1 0  0 "},
		{"é", "\x00a", "DSSEv1 2 é 2 \x00a"},
	} {
		if got := string(preAuthEncoding(vector.kind, []byte(vector.body))); got != vector.encoded {
			t.Fatalf("PAE = %q, want %q", got, vector.encoded)
		}
	}
}

func TestSignedAssignmentPreservesExactManifest(t *testing.T) {
	f := newSigningFixture(t)
	raw := f.signed(t)
	value, receipt, err := Accept(raw, f.trust, Receipt{}, f.now)
	if err != nil || !bytes.Equal(value.Manifest, f.value.Manifest) || receipt.Revision != 1 || receipt.PayloadSHA256 == "" {
		t.Fatalf("signed manifest changed or rejected: %v, receipt=%+v", err, receipt)
	}
	if _, repeated, err := Accept(raw, f.trust, receipt, f.now); err != nil || repeated != receipt {
		t.Fatalf("exact retry rejected: %v", err)
	}
}

func TestDSSEEnvelopeAcceptsBothBase64AlphabetsAndIgnoresKeyHint(t *testing.T) {
	f := newSigningFixture(t)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		var value envelope
		if err := json.Unmarshal(f.signed(t), &value); err != nil {
			t.Fatal(err)
		}
		payload, _ := base64.StdEncoding.DecodeString(value.Payload)
		signed, _ := base64.StdEncoding.DecodeString(value.Signatures[0].Value)
		value.Payload = encoding.EncodeToString(payload)
		value.Signatures[0] = signature{KeyID: "untrusted-hint", Value: encoding.EncodeToString(signed)}
		var extended map[string]any
		if err := json.Unmarshal(encodeJSON(t, value), &extended); err != nil {
			t.Fatal(err)
		}
		extended["extension"] = "ignored metadata"
		if _, _, err := Accept(encodeJSON(t, extended), f.trust, Receipt{}, f.now); err != nil {
			t.Fatalf("valid DSSE transport rejected: %v", err)
		}
	}
}

func TestDSSERejectsUnauthenticatedOrReinterpretedPayload(t *testing.T) {
	for name, change := range map[string]func(*envelope){
		"changed-type":        func(e *envelope) { e.PayloadType = "application/json" },
		"changed-body":        func(e *envelope) { e.Payload = base64.StdEncoding.EncodeToString([]byte(`{}`)) },
		"bad-payload-base64":  func(e *envelope) { e.Payload = "%" },
		"bad-signature":       func(e *envelope) { e.Signatures[0].Value = "AA==" },
		"missing-signature":   func(e *envelope) { e.Signatures = nil },
		"too-many-signatures": func(e *envelope) { e.Signatures = make([]signature, 9) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newSigningFixture(t)
			var value envelope
			if err := json.Unmarshal(f.signed(t), &value); err != nil {
				t.Fatal(err)
			}
			change(&value)
			if _, _, err := Accept(encodeJSON(t, value), f.trust, Receipt{}, f.now); err == nil {
				t.Fatal("unauthenticated envelope accepted")
			}
		})
	}
}

func TestSigningAndVerificationRejectInvalidKeys(t *testing.T) {
	f := newSigningFixture(t)
	broken := bytes.Clone(f.key)
	broken[len(broken)-1] ^= 1
	for _, key := range []ed25519.PrivateKey{nil, f.key[:31], broken} {
		if _, err := Sign(f.value, key, f.now); err == nil {
			t.Fatal("invalid private key accepted")
		}
	}
	raw := f.signed(t)
	for _, key := range []ed25519.PublicKey{nil, f.trust.PublicKey[:31], bytes.Repeat([]byte{22}, 32)} {
		f.trust.PublicKey = key
		if _, _, err := Accept(raw, f.trust, Receipt{}, f.now); err == nil {
			t.Fatal("invalid or unrelated verification key accepted")
		}
	}
}
