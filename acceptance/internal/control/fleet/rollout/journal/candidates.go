package journal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/fleet/registry"
	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func candidate(raw []byte) (wire.Candidate, error) {
	data, err := manifest.Decode(raw)
	if err != nil {
		return wire.Candidate{}, fmt.Errorf("%w: candidate manifest", registry.ErrInvalid)
	}
	return wire.Candidate{NodeID: data.NodeID, SHA256: contract.Digest(raw), Generation: data.Generation}, nil
}

func (s *Store) Stage(nodeID string, raw []byte) (wire.Candidate, error) {
	value, err := candidate(raw)
	if err != nil || value.NodeID != nodeID {
		return wire.Candidate{}, fmt.Errorf("%w: invalid or foreign candidate", registry.ErrInvalid)
	}
	path := filepath.Join(s.root, "candidates", value.SHA256+".json")
	if err := safeio.Create(path, raw, 0600); err != nil {
		if !os.IsExist(err) {
			return wire.Candidate{}, err
		}
		_, stored, err := s.Candidate(value.SHA256)
		if err != nil || !bytes.Equal(stored, raw) {
			return wire.Candidate{}, fmt.Errorf("existing candidate differs or is unreadable")
		}
	}
	return value, nil
}

func (s *Store) Candidate(hash string) (wire.Candidate, []byte, error) {
	if !validate.Hash(hash) {
		return wire.Candidate{}, nil, registry.ErrInvalid
	}
	path := filepath.Join(s.root, "candidates", hash+".json")
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return wire.Candidate{}, nil, err
	}
	raw, err := safeio.Read(path, contract.MaxManifestSize)
	if err != nil || contract.Digest(raw) != hash {
		return wire.Candidate{}, nil, fmt.Errorf("candidate checksum differs or cannot be read")
	}
	value, err := candidate(raw)
	return value, raw, err
}
