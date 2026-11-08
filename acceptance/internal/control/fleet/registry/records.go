package registry

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func (s *Store) readNode(nodeID string) (nodeRecord, error) {
	if validate.Name(nodeID) != nil {
		return nodeRecord{}, ErrInvalid
	}
	var value nodeRecord
	if err := readPrivate(filepath.Join(s.directory, "nodes", nodeID+".json"), &value); err != nil {
		if os.IsNotExist(err) {
			return nodeRecord{}, ErrNotFound
		}
		return nodeRecord{}, err
	}
	if value.NodeID != nodeID || value.FleetID != s.authority.FleetID {
		return nodeRecord{}, fmt.Errorf("Fleet record identity mismatch")
	}
	return value, validateRecord(value)
}

func (s *Store) saveNode(value nodeRecord) error {
	if err := validateRecord(value); err != nil {
		return err
	}
	return safeio.JSON(filepath.Join(s.directory, "nodes", value.NodeID+".json"), value)
}

func (s *Store) storeAssignment(nodeID string, raw []byte) error {
	directory := filepath.Join(s.directory, "assignments", nodeID)
	if err := os.Mkdir(directory, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := safeio.CheckAccess(directory, os.ModeDir|0700, os.Geteuid(), -1); err != nil {
		return err
	}
	path := filepath.Join(directory, contract.Digest(raw)+".json")
	if err := safeio.Create(path, raw, 0600); err != nil {
		if !os.IsExist(err) {
			return err
		}
		existing, err := readEnvelope(path)
		if err != nil || !bytes.Equal(existing, raw) {
			return fmt.Errorf("existing Fleet assignment differs or cannot be read")
		}
	}
	return safeio.SyncDir(filepath.Dir(directory))
}

func readEnvelope(path string) ([]byte, error) {
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return nil, err
	}
	return safeio.Read(path, contract.MaxEnvelopeSize)
}

func (s *Store) readAssignment(value nodeRecord) ([]byte, contract.Assignment, error) {
	desired := value.Desired
	directory := filepath.Join(s.directory, "assignments", value.NodeID)
	if err := safeio.CheckAccess(directory, os.ModeDir|0700, os.Geteuid(), -1); err != nil {
		return nil, contract.Assignment{}, err
	}
	raw, err := readEnvelope(filepath.Join(directory, desired.EnvelopeSHA256+".json"))
	if err != nil || contract.Digest(raw) != desired.EnvelopeSHA256 {
		return nil, contract.Assignment{}, fmt.Errorf("stored Fleet assignment checksum mismatch or unavailable")
	}
	trust := contract.Trust{FleetID: value.FleetID, NodeID: value.NodeID, PublicKey: s.key.Public().(ed25519.PublicKey)}
	// Authenticate archival bytes at their recorded issuance time. Clients always
	// check their current clock; expired archives are renewed before delivery.
	assignment, receipt, err := contract.Accept(raw, trust, contract.Receipt{}, time.Unix(desired.IssuedAt, 0))
	if err != nil || receipt.Revision != desired.Revision || receipt.PayloadSHA256 != desired.PayloadSHA256 || assignment.IssuedAt != desired.IssuedAt || assignment.ExpiresAt != desired.ExpiresAt || assignment.ManifestSHA256 != desired.ManifestSHA256 {
		return nil, contract.Assignment{}, fmt.Errorf("stored Fleet assignment metadata mismatch")
	}
	return raw, assignment, nil
}
