package rollout

import (
	"errors"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout/journal"
	"vpn-script/internal/core/fleet/wire"
)

func fail(record *journal.Record, reason, nodeID string) {
	record.Phase, record.Failure, record.FailedNode = "failed", reason, nodeID
}

func nodeFailure(err error) string {
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return "node_missing"
	case errors.Is(err, registry.ErrRevoked):
		return "node_revoked"
	case errors.Is(err, registry.ErrConflict):
		return "publication_conflict"
	case errors.Is(err, registry.ErrInvalid):
		return "candidate_invalid"
	}
	return ""
}

func (m *Manager) publish(record *journal.Record, index int, now int64) error {
	spec, progress := record.Spec.Stages[record.Stage].Targets[index], &record.Targets[record.Stage][index]
	node, err := m.backend.Node(spec.NodeID)
	if err == nil {
		err = eligibleNode(record.FleetID, node)
	}
	if err != nil {
		return m.publicationFailure(record, spec.NodeID, err, now)
	}
	desired := node.Desired
	if desired.OriginRequestID() != progress.RequestID {
		meta, raw, err := m.journal.Candidate(spec.CandidateSHA256)
		if err != nil || meta.NodeID != spec.NodeID || meta.Generation != progress.Generation {
			return m.publicationFailure(record, spec.NodeID, registry.ErrInvalid, now)
		}
		desired, err = m.backend.Publish(spec.NodeID, wire.Publish{RequestID: progress.RequestID,
			ExpectedRevision: spec.ExpectedRevision, ValiditySeconds: record.Spec.ValiditySeconds, Manifest: raw})
		if err != nil {
			return m.publicationFailure(record, spec.NodeID, err, now)
		}
	}
	if desired.ManifestSHA256 != spec.CandidateSHA256 || desired.Generation != progress.Generation || desired.OriginRequestID() != progress.RequestID || desired.Revision <= spec.ExpectedRevision {
		return m.publicationFailure(record, spec.NodeID, registry.ErrConflict, now)
	}
	progress.PublishedRevision, progress.Phase = desired.Revision, "waiting"
	return m.commit(record, now)
}

func (m *Manager) publicationFailure(record *journal.Record, nodeID string, err error, now int64) error {
	if reason := nodeFailure(err); reason != "" {
		fail(record, reason, nodeID)
		return m.commit(record, now)
	}
	// An ambiguous filesystem error may have committed the assignment. Stop the
	// coordinator; on reopen the persisted request identity recovers that result.
	return err
}

func eligibleNode(fleetID string, node wire.Node) error {
	if node.Revoked {
		return registry.ErrRevoked
	}
	if node.FleetID != fleetID || !node.Enrolled {
		return registry.ErrConflict
	}
	return nil
}
