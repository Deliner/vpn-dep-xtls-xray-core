package credential

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"

	"vpn-script/internal/platform/safeio"
)

// NewToken returns a canonical URL-safe bearer credential with 256 random bits.
func NewToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func ValidToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == token
}

func TokenHash(token string) string { return safeio.Hash([]byte(token)) }

func Matches(hash, token string) bool {
	return ValidToken(token) && subtle.ConstantTimeCompare([]byte(hash), []byte(TokenHash(token))) == 1
}
