package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/core/fleet/wire"
)

func (s *Store) UpdateStatus(nodeID, token string, report wire.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	value, err := s.authorizeNode(nodeID, token)
	if err != nil {
		return err
	}
	if err := report.Validate(value.FleetID, value.NodeID, value.Desired.Revision); err != nil {
		return ErrInvalid
	}
	if previous := value.Status; previous != nil {
		if report.Attempt < previous.Attempt || report.AcceptedRevision < previous.AcceptedRevision || report.Attempt == previous.Attempt && report != *previous {
			return ErrConflict
		}
	}
	now := max(value.LastSeen, s.now().Unix())
	if value.Status == nil || report.Attempt > value.Status.Attempt {
		value.StatusReceivedAt = now
	}
	value.Status, value.LastSeen = &report, now
	return s.saveNode(value)
}

func (s *Store) Node(nodeID string) (wire.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return wire.Node{}, err
	}
	value, err := s.readNode(nodeID)
	return value.view(), err
}

func (s *Store) List() ([]wire.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(s.directory, "nodes"))
	if err != nil {
		return nil, err
	}
	result := make([]wire.Node, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".nhop-") && entry.Type().IsRegular() {
			continue // An interrupted atomic write is not a published node record.
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, fmt.Errorf("unexpected entry in Fleet node registry")
		}
		value, err := s.readNode(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		result = append(result, value.view())
	}
	return result, nil
}

func (value nodeRecord) view() wire.Node {
	return wire.Node{FleetID: value.FleetID, NodeID: value.NodeID, Group: value.Group,
		Enrolled: value.EnrolledAt != 0, Revoked: value.RevokedAt != 0, Desired: value.Desired,
		Status: value.Status, LastSeen: value.LastSeen, StatusReceivedAt: value.StatusReceivedAt}
}
