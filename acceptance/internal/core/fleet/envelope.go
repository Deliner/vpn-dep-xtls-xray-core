package fleet

import (
	"encoding/base64"
	"fmt"
)

type envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []signature `json:"signatures"`
}

type signature struct {
	KeyID string `json:"keyid,omitempty"`
	Value string `json:"sig"`
}

// preAuthEncoding is DSSE v1 PAE. Lengths count bytes, including for UTF-8 types.
func preAuthEncoding(payloadType string, payload []byte) []byte {
	prefix := fmt.Sprintf("DSSEv1 %d %s %d ", len(payloadType), payloadType, len(payload))
	return append([]byte(prefix), payload...)
}

func decodeBase64(value string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.Strict().DecodeString(value)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 in signed envelope")
}
