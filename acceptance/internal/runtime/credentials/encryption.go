package credentials

import (
	"crypto/mlkem"
	"encoding/base64"
	"fmt"
	"vpn-script/internal/core/model"
)

func encryptionSecret(secret model.Secret) (model.Secret, error) {
	key, err := mlkem.GenerateKey768()
	if err != nil {
		return secret, err
	}
	secret.EncryptionSeed = base64.RawURLEncoding.EncodeToString(key.Bytes())
	secret.EncryptionKey = base64.RawURLEncoding.EncodeToString(key.EncapsulationKey().Bytes())
	return secret, nil
}

func validateEncryption(secret model.Secret) error {
	raw, err := base64.RawURLEncoding.DecodeString(secret.EncryptionSeed)
	if err != nil {
		return fmt.Errorf("invalid VLESS encryption seed")
	}
	key, err := mlkem.NewDecapsulationKey768(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(key.EncapsulationKey().Bytes()) != secret.EncryptionKey {
		return fmt.Errorf("VLESS encryption key pair mismatch")
	}
	return nil
}
