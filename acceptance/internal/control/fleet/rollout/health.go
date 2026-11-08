package rollout

import (
	"vpn-script/internal/control/fleet/rollout/journal"
	"vpn-script/internal/core/fleet/wire"
)

func (m *Manager) observe(record *journal.Record, now int64) error {
	gap := now-record.UpdatedAt > record.Spec.FreshSeconds
	for stage := 0; stage <= record.Stage; stage++ {
		for index := range record.Targets[stage] {
			progress := &record.Targets[stage][index]
			if progress.PublishedRevision == 0 {
				continue
			}
			if gap {
				waiting(progress)
			}
			if err := m.observeTarget(record, stage, index, now); err != nil || record.Phase == "failed" {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) observeTarget(record *journal.Record, stage, index int, now int64) error {
	spec, progress := record.Spec.Stages[stage].Targets[index], &record.Targets[stage][index]
	node, err := m.backend.Node(spec.NodeID)
	if err == nil {
		err = eligibleNode(record.FleetID, node)
	}
	if err != nil {
		if reason := nodeFailure(err); reason != "" {
			fail(record, reason, spec.NodeID)
			return nil
		}
		return err
	}
	if node.Desired.OriginRequestID() != progress.RequestID || node.Desired.ManifestSHA256 != spec.CandidateSHA256 || node.Desired.Generation != progress.Generation || node.Desired.Revision < progress.PublishedRevision {
		fail(record, "publication_conflict", spec.NodeID)
		return nil
	}
	status := node.Status
	if status != nil && status.AcceptedRevision >= progress.PublishedRevision && status.Phase == "failed" {
		fail(record, "install_failed", spec.NodeID)
		return nil
	}
	if status != nil && status.AppliedRevision >= progress.PublishedRevision && status.Generation != progress.Generation {
		fail(record, "generation_mismatch", spec.NodeID)
		return nil
	}
	observeHealth(progress, node, record.Spec, now)
	return nil
}

func observeHealth(progress *journal.Target, node wire.Node, spec wire.RolloutSpec, now int64) {
	progress.LastSeen = node.LastSeen
	status := node.Status
	if status == nil || status.Phase != "ready" || status.AppliedRevision < progress.PublishedRevision || node.StatusReceivedAt <= 0 || node.StatusReceivedAt > now || now-node.StatusReceivedAt > spec.FreshSeconds {
		waiting(progress)
		return
	}
	if progress.HealthySince == 0 || status.Attempt < progress.FirstReadyAttempt {
		progress.HealthySince, progress.FirstReadyAttempt = now, status.Attempt
	}
	progress.Phase = "observing"
	if node.StatusReceivedAt-progress.HealthySince >= spec.ObserveSeconds && status.Attempt > progress.FirstReadyAttempt {
		progress.Phase = "ready"
	}
}

func waiting(progress *journal.Target) {
	progress.Phase, progress.HealthySince, progress.FirstReadyAttempt = "waiting", 0, 0
}

func stagesReady(record journal.Record, through int) bool {
	for stage := 0; stage <= through; stage++ {
		for _, target := range record.Targets[stage] {
			if target.Phase != "ready" {
				return false
			}
		}
	}
	return true
}
