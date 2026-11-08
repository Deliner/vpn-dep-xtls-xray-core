package pull

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/node/fleet"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

// Initialize never overwrites an existing or partially provisioned directory.
// Its parent must already be protected against replacement by other users.
func Initialize(directory string, bootstrap Bootstrap) error {
	value, err := initialConnection(bootstrap)
	if err != nil {
		return err
	}
	client, err := fleetapi.New(value.Endpoint, value.CAPEM)
	if err != nil {
		return err
	}
	client.Close()
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	if err := fleet.New(filepath.Join(directory, "agent"), value.Trust).Initialize(); err != nil {
		return err
	}
	if err := safeio.NewJSON(filepath.Join(directory, "connection.json"), value); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(directory))
}

func initialConnection(bootstrap Bootstrap) (connection, error) {
	invitation := bootstrap.Invitation
	trust := contract.Trust{FleetID: invitation.FleetID, NodeID: invitation.NodeID, PublicKey: invitation.PublicKey}
	if bootstrap.Version != 1 || trust.Validate() != nil || !credential.ValidToken(invitation.Token) || invitation.ExpiresAt <= time.Now().Unix() {
		return connection{}, fmt.Errorf("invalid or expired Fleet bootstrap")
	}
	token, err := credential.NewToken()
	if err != nil {
		return connection{}, err
	}
	return connection{Version: 1, Endpoint: bootstrap.Endpoint, CAPEM: bootstrap.CAPEM, Trust: trust,
		Credential: token, EnrollmentToken: invitation.Token}, nil
}

func readConnection(directory string) (connection, error) {
	path := filepath.Join(directory, "connection.json")
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return connection{}, err
	}
	var value connection
	if err := safeio.LoadJSON(path, &value); err != nil {
		return connection{}, err
	}
	if value.Version != 1 || value.Trust.Validate() != nil || !credential.ValidToken(value.Credential) {
		return connection{}, fmt.Errorf("invalid Fleet connection identity")
	}
	if value.Enrolled && value.EnrollmentToken != "" || !value.Enrolled && !credential.ValidToken(value.EnrollmentToken) {
		return connection{}, fmt.Errorf("invalid Fleet enrollment phase")
	}
	return value, nil
}
