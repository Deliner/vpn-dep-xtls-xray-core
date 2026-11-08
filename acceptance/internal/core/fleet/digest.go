package fleet

import (
	"crypto/sha256"
	"encoding/hex"
)

func Digest(raw []byte) string {
	value := sha256.Sum256(raw)
	return hex.EncodeToString(value[:])
}
