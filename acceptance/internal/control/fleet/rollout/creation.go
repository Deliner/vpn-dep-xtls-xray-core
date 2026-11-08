package rollout

import (
	"errors"
	"fmt"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout/journal"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/safeio"
)

func (m *Manager) Stage(nodeID string, raw []byte) (wire.Candidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return wire.Candidate{}, err
	}
	node, err := m.backend.Node(nodeID)
	if err != nil {
		return wire.Candidate{}, err
	}
	if node.Revoked {
		return wire.Candidate{}, registry.ErrRevoked
	}
	return m.journal.Stage(nodeID, raw)
}

func (m *Manager) Create(spec wire.RolloutSpec) (wire.Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, existing, err := m.plan(spec)
	if err != nil {
		return wire.Rollout{}, err
	}
	if existing {
		return view(record, true), nil
	}
	return view(record, true), m.journal.Save(record, true)
}

// Preview shares creation checks but neither reserves nodes nor persists a plan.
// The confirmation must call Create, which checks the current state again.
func (m *Manager) Preview(spec wire.RolloutSpec) (wire.Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, _, err := m.plan(spec)
	if err != nil {
		return wire.Rollout{}, err
	}
	return view(record, true), nil
}

func (m *Manager) plan(spec wire.RolloutSpec) (journal.Record, bool, error) {
	if err := m.available(); err != nil {
		return journal.Record{}, false, err
	}
	if err := journal.ValidateSpec(spec); err != nil {
		return journal.Record{}, false, err
	}
	hash, err := safeio.Digest(spec)
	if err != nil {
		return journal.Record{}, false, err
	}
	if previous, err := m.journal.Read(spec.ID); err == nil {
		if previous.SpecSHA256 != hash {
			return journal.Record{}, false, registry.ErrConflict
		}
		return previous, true, nil
	} else if !errors.Is(err, registry.ErrNotFound) {
		return journal.Record{}, false, err
	}
	if err := m.unclaimed(spec); err != nil {
		return journal.Record{}, false, err
	}
	record, err := m.prepare(spec, hash)
	return record, false, err
}

func (m *Manager) unclaimed(spec wire.RolloutSpec) error {
	names, err := m.journal.Names()
	if err != nil {
		return err
	}
	if len(names) >= journal.MaxPlans {
		return fmt.Errorf("%w: rollout history limit reached", registry.ErrConflict)
	}
	wanted := map[string]bool{}
	for _, stage := range spec.Stages {
		for _, target := range stage.Targets {
			wanted[target.NodeID] = true
		}
	}
	active := 0
	for _, name := range names {
		record, err := m.journal.Read(name)
		if err != nil {
			return err
		}
		if record.Phase != "running" {
			continue
		}
		active++
		for _, stage := range record.Spec.Stages {
			for _, target := range stage.Targets {
				if wanted[target.NodeID] {
					return fmt.Errorf("%w: node belongs to an active rollout", registry.ErrConflict)
				}
			}
		}
	}
	if active >= 64 {
		return fmt.Errorf("%w: too many active rollouts", registry.ErrConflict)
	}
	return nil
}

func (m *Manager) prepare(spec wire.RolloutSpec, hash string) (journal.Record, error) {
	now := m.now().Unix()
	record := journal.Record{Version: 1, Spec: spec, SpecSHA256: hash, Phase: "running", CreatedAt: now, UpdatedAt: now, StageStartedAt: now}
	for _, stage := range spec.Stages {
		var targets []journal.Target
		for _, target := range stage.Targets {
			value, err := m.prepareTarget(&record, target)
			if err != nil {
				return journal.Record{}, err
			}
			targets = append(targets, value)
		}
		record.Targets = append(record.Targets, targets)
	}
	return record, nil
}

func (m *Manager) prepareTarget(record *journal.Record, target wire.RolloutTarget) (journal.Target, error) {
	meta, _, err := m.journal.Candidate(target.CandidateSHA256)
	if err != nil || meta.NodeID != target.NodeID {
		return journal.Target{}, fmt.Errorf("%w: candidate missing, invalid or belongs to another node", registry.ErrInvalid)
	}
	node, err := m.backend.Node(target.NodeID)
	if err != nil {
		return journal.Target{}, err
	}
	if record.FleetID == "" {
		record.FleetID = node.FleetID
	}
	if node.FleetID != record.FleetID || !node.Enrolled || node.Revoked || node.Desired.Revision != target.ExpectedRevision {
		return journal.Target{}, fmt.Errorf("%w: node identity, enrollment or expected revision", registry.ErrConflict)
	}
	return journal.Target{Generation: meta.Generation, RequestID: journal.RequestID(record.SpecSHA256, target.NodeID), Phase: "pending"}, nil
}
