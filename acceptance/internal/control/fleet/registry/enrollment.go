package registry

import (
	"bytes"
	"crypto/ed25519"
	"errors"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/credential"
)

func (s *Store) Invite(request wire.Invite) (wire.Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return wire.Invitation{}, err
	}
	if err := validateInvite(request); err != nil {
		return wire.Invitation{}, err
	}
	value, err := s.readNode(request.NodeID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return wire.Invitation{}, err
	}
	if value.EnrolledAt != 0 || value.RevokedAt != 0 {
		return wire.Invitation{}, ErrConflict
	}
	token, err := credential.NewToken()
	if err != nil {
		return wire.Invitation{}, err
	}
	value.Version, value.FleetID, value.NodeID, value.Group = 1, s.authority.FleetID, request.NodeID, request.Group
	value.EnrollmentHash, value.EnrollmentEnd = credential.TokenHash(token), s.now().Unix()+request.ValiditySeconds
	if err := s.saveNode(value); err != nil {
		return wire.Invitation{}, err
	}
	return wire.Invitation{FleetID: value.FleetID, NodeID: value.NodeID, Token: token,
		PublicKey: bytes.Clone(s.key[ed25519.SeedSize:]), ExpiresAt: value.EnrollmentEnd}, nil
}

func (s *Store) Enroll(request wire.Enrollment) (wire.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return wire.Identity{}, err
	}
	value, err := s.readNode(request.NodeID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		return wire.Identity{}, ErrUnauthorized
	}
	if err != nil {
		return wire.Identity{}, err
	}
	if err := s.acceptEnrollment(&value, request); err != nil {
		return wire.Identity{}, err
	}
	if err := s.saveNode(value); err != nil {
		return wire.Identity{}, err
	}
	return wire.Identity{FleetID: value.FleetID, NodeID: value.NodeID}, nil
}

func (s *Store) acceptEnrollment(value *nodeRecord, request wire.Enrollment) error {
	if !credential.Matches(value.EnrollmentHash, request.Token) || !credential.ValidToken(request.Credential) {
		return ErrUnauthorized
	}
	if value.RevokedAt != 0 {
		return ErrRevoked
	}
	if value.EnrolledAt != 0 {
		if !credential.Matches(value.CredentialHash, request.Credential) {
			return ErrUnauthorized
		}
		return nil
	}
	if s.now().Unix() >= value.EnrollmentEnd {
		return ErrUnauthorized
	}
	value.CredentialHash, value.EnrolledAt = credential.TokenHash(request.Credential), s.now().Unix()
	return nil
}

func (s *Store) Revoke(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	value, err := s.readNode(nodeID)
	if err != nil {
		return err
	}
	if value.RevokedAt != 0 {
		return nil
	}
	value.RevokedAt = s.now().Unix()
	return s.saveNode(value)
}

func (s *Store) authorizeNode(nodeID, token string) (nodeRecord, error) {
	value, err := s.readNode(nodeID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		return nodeRecord{}, ErrUnauthorized
	}
	if err != nil {
		return nodeRecord{}, err
	}
	if value.EnrolledAt == 0 || !credential.Matches(value.CredentialHash, token) {
		return nodeRecord{}, ErrUnauthorized
	}
	if value.RevokedAt != 0 {
		return nodeRecord{}, ErrRevoked
	}
	return value, nil
}
