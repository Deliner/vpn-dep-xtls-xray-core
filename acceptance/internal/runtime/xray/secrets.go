package xray

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/credentials"
)

func NewSecret(protocol string) (model.Secret, error) { return credentials.NewSecret(protocol) }
func ValidateSecret(secret model.Secret) error        { return credentials.ValidateSecret(secret) }
