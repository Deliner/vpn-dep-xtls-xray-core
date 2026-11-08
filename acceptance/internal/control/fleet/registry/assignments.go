package registry

import (
	"crypto/ed25519"
	"fmt"
	"math"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/credential"
)

func (s *Store) Publish(nodeID string, request wire.Publish) (wire.Desired, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, data, err := s.preparePublication(nodeID, request)
	if err != nil {
		return wire.Desired{}, err
	}
	if value.Desired.RequestID == request.RequestID {
		return repeatedPublication(value.Desired, request)
	}
	return s.publishManifest(value, request, data, request.RequestID)
}

// Preview validates the same request and current revision as Publish without
// signing, storing or changing desired state. Publish checks again on commit.
func (s *Store) Preview(nodeID string, request wire.Publish) (model.NodeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, data, err := s.preparePublication(nodeID, request)
	return data, err
}

func (s *Store) preparePublication(nodeID string, request wire.Publish) (nodeRecord, model.NodeConfig, error) {
	if err := s.available(); err != nil {
		return nodeRecord{}, model.NodeConfig{}, err
	}
	data, err := validatePublication(nodeID, request)
	if err != nil {
		return nodeRecord{}, data, err
	}
	value, err := s.readNode(nodeID)
	if err != nil {
		return value, data, err
	}
	if value.RevokedAt != 0 {
		return value, data, ErrRevoked
	}
	if value.Desired.RequestID == request.RequestID {
		_, err = repeatedPublication(value.Desired, request)
		return value, data, err
	}
	if value.Desired.Revision != request.ExpectedRevision {
		return value, data, ErrConflict
	}
	return value, data, nil
}

func validatePublication(nodeID string, request wire.Publish) (model.NodeConfig, error) {
	if validate.Name(nodeID) != nil || !validate.Hash(request.RequestID) || request.ExpectedRevision == math.MaxUint64 || request.ValiditySeconds < 1 || request.ValiditySeconds > contract.MaxValiditySeconds {
		return model.NodeConfig{}, ErrInvalid
	}
	data, err := manifest.Decode(request.Manifest)
	if err != nil || data.NodeID != nodeID {
		return model.NodeConfig{}, fmt.Errorf("%w: invalid or foreign node manifest", ErrInvalid)
	}
	return data, nil
}

func repeatedPublication(desired wire.Desired, request wire.Publish) (wire.Desired, error) {
	if desired.Revision != request.ExpectedRevision+1 || desired.ManifestSHA256 != contract.Digest(request.Manifest) || desired.ExpiresAt-desired.IssuedAt != request.ValiditySeconds {
		return wire.Desired{}, ErrConflict
	}
	return desired, nil
}

func (s *Store) publishManifest(value nodeRecord, request wire.Publish, data model.NodeConfig, origin string) (wire.Desired, error) {
	raw, desired, err := s.buildAssignment(value, request, data.Generation)
	if err != nil {
		return wire.Desired{}, err
	}
	desired.PublicationID = origin
	if err := s.storeAssignment(value.NodeID, raw); err != nil {
		return wire.Desired{}, err
	}
	value.Desired = desired
	if err := s.saveNode(value); err != nil {
		// Keep the immutable artifact: rename may have committed before fsync
		// failed, so deleting it could break an already-visible desired pointer.
		return wire.Desired{}, err
	}
	return desired, nil
}

func (s *Store) buildAssignment(value nodeRecord, request wire.Publish, generation string) ([]byte, wire.Desired, error) {
	now := s.now()
	assignment, err := contract.NewAssignment(value.FleetID, value.NodeID, value.Desired.Revision+1, request.Manifest, now, time.Duration(request.ValiditySeconds)*time.Second)
	if err != nil {
		return nil, wire.Desired{}, err
	}
	raw, err := contract.Sign(assignment, s.key, now)
	if err != nil {
		return nil, wire.Desired{}, err
	}
	trust := contract.Trust{FleetID: value.FleetID, NodeID: value.NodeID, PublicKey: s.key.Public().(ed25519.PublicKey)}
	_, receipt, err := contract.Accept(raw, trust, contract.Receipt{}, now)
	if err != nil {
		return nil, wire.Desired{}, err
	}
	desired := wire.Desired{Revision: receipt.Revision, Generation: generation, EnvelopeSHA256: contract.Digest(raw),
		PayloadSHA256: receipt.PayloadSHA256, ManifestSHA256: assignment.ManifestSHA256, RequestID: request.RequestID,
		IssuedAt: assignment.IssuedAt, ExpiresAt: assignment.ExpiresAt}
	return raw, desired, nil
}

func (s *Store) Assignment(nodeID, token string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	value, err := s.authorizeNode(nodeID, token)
	if err != nil {
		return nil, err
	}
	if value.Desired.Revision == 0 {
		return nil, nil
	}
	raw, assignment, err := s.readAssignment(value)
	if err != nil {
		return nil, err
	}
	validity := assignment.ExpiresAt - assignment.IssuedAt
	if s.now().Unix() < assignment.ExpiresAt-min(validity/2, 3600) {
		return raw, nil
	}
	return s.renewAssignment(value, assignment, validity)
}

func (s *Store) renewAssignment(value nodeRecord, previous contract.Assignment, validity int64) ([]byte, error) {
	nonce, err := credential.NewToken()
	if err != nil {
		return nil, err
	}
	request := wire.Publish{RequestID: credential.TokenHash(nonce), ExpectedRevision: value.Desired.Revision,
		ValiditySeconds: validity, Manifest: previous.Manifest}
	data, err := validatePublication(value.NodeID, request)
	if err != nil || data.Generation != value.Desired.Generation {
		return nil, fmt.Errorf("stored Fleet manifest no longer matches installer policy")
	}
	desired, err := s.publishManifest(value, request, data, value.Desired.OriginRequestID())
	if err != nil {
		return nil, err
	}
	value.Desired = desired
	raw, _, err := s.readAssignment(value)
	return raw, err
}
