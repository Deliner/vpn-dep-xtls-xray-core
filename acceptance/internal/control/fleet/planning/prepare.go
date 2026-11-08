package planning

import (
	"context"
	"fmt"
	"path/filepath"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func Prepare(ctx context.Context, remote Remote, token, path string) (wire.RolloutSpec, error) {
	spec, inputs, err := inspect(path)
	if err != nil {
		return wire.RolloutSpec{}, err
	}
	if err := snapshot(ctx, remote, token, &spec); err != nil {
		return wire.RolloutSpec{}, err
	}
	for _, input := range inputs {
		raw, err := privateBytes(input.path)
		if err != nil || safeio.Hash(raw) != input.meta.SHA256 {
			return wire.RolloutSpec{}, fmt.Errorf("local candidate changed during planning")
		}
		meta, err := remote.StageCandidate(ctx, input.meta.NodeID, token, raw)
		if err != nil {
			return wire.RolloutSpec{}, err
		}
		if meta != input.meta {
			return wire.RolloutSpec{}, fmt.Errorf("controller staged different candidate bytes or identity")
		}
	}
	return spec, nil
}

func inspect(path string) (wire.RolloutSpec, []input, error) {
	value, err := readDraft(path)
	if err != nil {
		return wire.RolloutSpec{}, nil, err
	}
	spec := wire.RolloutSpec{Version: value.Version, ID: value.ID, ObserveSeconds: value.ObserveSeconds,
		FreshSeconds: value.FreshSeconds, StageTimeoutSeconds: value.StageTimeoutSeconds, ValiditySeconds: value.ValiditySeconds}
	var inputs []input
	for _, group := range value.Stages {
		stage := wire.RolloutStage{Name: group.Name}
		for _, name := range group.Manifests {
			input, err := inspectManifest(path, name)
			if err != nil {
				return wire.RolloutSpec{}, nil, err
			}
			inputs = append(inputs, input)
			if len(inputs) > wire.MaxRolloutNodes {
				return wire.RolloutSpec{}, nil, fmt.Errorf("too many rollout manifests")
			}
			stage.Targets = append(stage.Targets, wire.RolloutTarget{NodeID: input.meta.NodeID, CandidateSHA256: input.meta.SHA256})
		}
		spec.Stages = append(spec.Stages, stage)
	}
	return spec, inputs, spec.Validate()
}

func inspectManifest(draftPath, name string) (input, error) {
	if name == "" {
		return input{}, fmt.Errorf("empty rollout manifest path")
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(draftPath), path)
	}
	raw, err := privateBytes(path)
	if err != nil {
		return input{}, err
	}
	data, err := manifest.Decode(raw)
	if err != nil {
		return input{}, fmt.Errorf("invalid rollout manifest: %w", err)
	}
	return input{path: path, meta: wire.Candidate{NodeID: data.NodeID, SHA256: safeio.Hash(raw), Generation: data.Generation}}, nil
}

func snapshot(ctx context.Context, remote Remote, token string, spec *wire.RolloutSpec) error {
	fleetID := ""
	for stage := range spec.Stages {
		for index := range spec.Stages[stage].Targets {
			target := &spec.Stages[stage].Targets[index]
			node, err := remote.Node(ctx, target.NodeID, token)
			if err != nil {
				return err
			}
			if fleetID == "" {
				fleetID = node.FleetID
			}
			if node.NodeID != target.NodeID || !node.Enrolled || node.Revoked || node.FleetID != fleetID {
				return fmt.Errorf("rollout requires enrolled nodes from the same fleet")
			}
			target.ExpectedRevision = node.Desired.Revision
		}
	}
	return spec.Validate()
}
