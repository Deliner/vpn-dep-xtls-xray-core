package rollout

import (
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout/journal"
	"vpn-script/internal/core/fleet/wire"
)

func view(record journal.Record, detail bool) wire.Rollout {
	value := wire.Rollout{ID: record.Spec.ID, FleetID: record.FleetID, SpecSHA256: record.SpecSHA256, Phase: record.Phase,
		Stage: record.Stage, StageName: record.Spec.Stages[record.Stage].Name, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		StageStartedAt: record.StageStartedAt, Failure: record.Failure, FailedNode: record.FailedNode}
	for stage, targets := range record.Targets {
		for index, target := range targets {
			value.TotalNodes++
			if target.PublishedRevision != 0 {
				value.PublishedNodes++
			}
			if target.Phase == "ready" {
				value.ReadyNodes++
			}
			if detail {
				value.Nodes = append(value.Nodes, wire.RolloutNode{NodeID: record.Spec.Stages[stage].Targets[index].NodeID,
					Stage: stage, Generation: target.Generation, PublishedRevision: target.PublishedRevision,
					Phase: target.Phase, LastSeen: target.LastSeen, HealthySince: target.HealthySince})
			}
		}
	}
	return value
}

func (m *Manager) Get(id string) (wire.Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return wire.Rollout{}, err
	}
	record, err := m.journal.Read(id)
	if err != nil {
		return wire.Rollout{}, err
	}
	return view(record, true), nil
}

func (m *Manager) List() ([]wire.Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return nil, err
	}
	names, err := m.journal.Names()
	if err != nil {
		return nil, err
	}
	values := make([]wire.Rollout, 0, len(names))
	for _, id := range names {
		record, err := m.journal.Read(id)
		if err != nil {
			return nil, err
		}
		values = append(values, view(record, false))
	}
	return values, nil
}

func (m *Manager) Cancel(id string) (wire.Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return wire.Rollout{}, err
	}
	record, err := m.journal.Read(id)
	if err != nil {
		return wire.Rollout{}, err
	}
	if record.Phase == "canceled" {
		return view(record, true), nil
	}
	if record.Phase != "running" {
		return wire.Rollout{}, registry.ErrConflict
	}
	record.Phase = "canceled"
	if err := m.commit(&record, m.now().Unix()); err != nil {
		return wire.Rollout{}, err
	}
	return view(record, true), nil
}

func (m *Manager) commit(record *journal.Record, now int64) error {
	record.UpdatedAt = max(record.UpdatedAt, now)
	return m.journal.Save(*record, false)
}
