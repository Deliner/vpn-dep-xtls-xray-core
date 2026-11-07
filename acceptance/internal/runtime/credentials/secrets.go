package credentials

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func NewSecret(protocol string) (model.Secret, error) {
	secret := model.Secret{Protocol: protocol}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return secret, err
	}
	secret.Password = Password()
	secret.BridgePassword = Password()
	secret.ObfsPassword = Password()
	random[6], random[8] = (random[6]&15)|64, (random[8]&63)|128
	secret.UUID = fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:16])
	if protocol == "vless-encryption" {
		return encryptionSecret(secret)
	}
	if protocol != model.Reality {
		return secret, nil
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return secret, err
	}
	secret.ShortID = hex.EncodeToString(random[16:24])
	secret.PrivateKey = base64.RawURLEncoding.EncodeToString(key.Bytes())
	secret.PublicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	return secret, nil
}

func ValidateSecret(secret model.Secret) error {
	if secret.Protocol == "vless-encryption" {
		if err := validatePassword(secret); err != nil {
			return err
		}
		return validateEncryption(secret)
	}
	if secret.Protocol == model.Shadowsocks {
		key, err := base64.StdEncoding.DecodeString(secret.Password)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("invalid saved Shadowsocks key")
		}
		return nil
	}
	if spec, ok := protocol.Lookup(secret.Protocol); !ok || spec.Status != "implemented" {
		return fmt.Errorf("invalid saved protocol")
	}
	if secret.Protocol != model.Reality {
		return validatePassword(secret)
	}
	if !uuidPattern.MatchString(secret.UUID) {
		return fmt.Errorf("invalid saved protocol/UUID")
	}
	short, err := hex.DecodeString(secret.ShortID)
	if err != nil || len(short) != 8 {
		return fmt.Errorf("invalid saved short ID")
	}
	raw, err := base64.RawURLEncoding.DecodeString(secret.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid saved private key")
	}
	key, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()) != secret.PublicKey {
		return fmt.Errorf("saved REALITY key pair does not match")
	}
	return nil
}

func Password() string {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(raw[:])
}

func validatePassword(secret model.Secret) error {
	if len(secret.Password) < 20 || !uuidPattern.MatchString(secret.UUID) {
		return fmt.Errorf("invalid saved credentials")
	}
	return nil
}
