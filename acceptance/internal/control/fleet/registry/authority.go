package registry

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

// Initialize creates a new private registry. It never repairs or overwrites an
// existing directory, including an interrupted bootstrap.
func Initialize(directory, fleetID string) error {
	if validate.Name(fleetID) != nil {
		return fmt.Errorf("%w: fleet ID", ErrInvalid)
	}
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	token, err := credential.NewToken()
	if err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	for _, name := range []string{"nodes", "assignments", "rollouts"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0700); err != nil {
			return err
		}
	}
	if err := safeio.NewJSON(filepath.Join(directory, "admin-credential.json"), adminCredential{Token: token}); err != nil {
		return err
	}
	value := authority{Version: 1, FleetID: fleetID, Seed: key.Seed(), AdminHash: credential.TokenHash(token)}
	if err := safeio.NewJSON(filepath.Join(directory, "authority.json"), value); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(directory))
}

func readPrivate(path string, value any) error {
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return err
	}
	return safeio.LoadJSON(path, value)
}

func readAuthority(directory string) (authority, error) {
	var value authority
	if err := readPrivate(filepath.Join(directory, "authority.json"), &value); err != nil {
		return authority{}, err
	}
	if value.Version != 1 || validate.Name(value.FleetID) != nil || len(value.Seed) != ed25519.SeedSize || !validate.Hash(value.AdminHash) {
		return authority{}, fmt.Errorf("invalid Fleet authority")
	}
	return value, nil
}

func ReadAdminCredential(path string) (string, error) {
	var value adminCredential
	if err := readPrivate(path, &value); err != nil {
		return "", err
	}
	if !credential.ValidToken(value.Token) {
		return "", fmt.Errorf("invalid Fleet administrator credential")
	}
	return value.Token, nil
}
