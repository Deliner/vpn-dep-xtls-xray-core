package app

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"vpn-script/internal/control/fleet/planning"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

func admin(ctx context.Context, command string, o options, output io.Writer) error {
	client, token, ca, err := adminClient(o)
	if err != nil {
		return err
	}
	defer client.Close()
	if command == "candidate" || strings.HasPrefix(command, "rollout") {
		return rolloutAdmin(ctx, command, client, token, o, output)
	}
	switch command {
	case "invite":
		return invite(ctx, client, token, ca, o, output)
	case "publish":
		var request wire.Publish
		if err := privateRead(o.request, &request); err != nil {
			return err
		}
		data, err := manifest.Decode(request.Manifest)
		if err != nil {
			return err
		}
		result, err := client.Publish(ctx, data.NodeID, token, request)
		return emit(output, result, err)
	case "revoke":
		return emit(output, map[string]string{"revoked": o.node}, client.Revoke(ctx, o.node, token))
	default:
		result, err := client.List(ctx, token)
		return emit(output, result, err)
	}
}

func adminClient(o options) (*fleetapi.Client, string, []byte, error) {
	if err := protectedParent(o.credential); err != nil {
		return nil, "", nil, err
	}
	token, err := registry.ReadAdminCredential(o.credential)
	if err != nil {
		return nil, "", nil, err
	}
	var ca []byte
	if o.ca != "" {
		ca, err = safeio.Read(o.ca, 1<<20)
		if err != nil {
			return nil, "", nil, err
		}
	}
	client, err := fleetapi.New(o.endpoint, ca)
	return client, token, ca, err
}

func invite(ctx context.Context, client *fleetapi.Client, token string, ca []byte, o options, output io.Writer) error {
	if err := newOutput(o.output); err != nil {
		return err
	}
	value, err := client.Invite(ctx, token, wire.Invite{NodeID: o.node, Group: o.group, ValiditySeconds: o.validity})
	if err != nil {
		return err
	}
	bootstrap := pull.Bootstrap{Version: 1, Endpoint: o.endpoint, CAPEM: ca, Invitation: value}
	if err := safeio.NewJSON(o.output, bootstrap); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"node": value.NodeID, "bootstrap": o.output})
}

func emit(output io.Writer, value any, err error) error {
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(value)
}

func rolloutAdmin(ctx context.Context, command string, client *fleetapi.Client, token string, o options, output io.Writer) error {
	switch command {
	case "rollout-plan":
		return rolloutPlan(ctx, client, token, o, output)
	case "candidate":
		raw, err := privateBytes(o.manifest)
		if err != nil {
			return err
		}
		data, err := manifest.Decode(raw)
		if err != nil {
			return err
		}
		value, err := client.StageCandidate(ctx, data.NodeID, token, raw)
		return emit(output, value, err)
	case "rollout-create":
		var spec wire.RolloutSpec
		if err := privateRead(o.request, &spec); err != nil {
			return err
		}
		value, err := client.CreateRollout(ctx, token, spec)
		return emit(output, value, err)
	case "rollout":
		value, err := client.Rollout(ctx, o.rollout, token)
		return emit(output, value, err)
	case "rollout-cancel":
		value, err := client.CancelRollout(ctx, o.rollout, token)
		return emit(output, value, err)
	default:
		value, err := client.Rollouts(ctx, token)
		return emit(output, value, err)
	}
}

func rolloutPlan(ctx context.Context, client *fleetapi.Client, token string, o options, output io.Writer) error {
	if err := newOutput(o.output); err != nil {
		return err
	}
	spec, err := planning.Prepare(ctx, client, token, o.request)
	if err != nil {
		return err
	}
	if err := safeio.NewJSON(o.output, spec); err != nil {
		return err
	}
	return emit(output, map[string]string{"rollout": spec.ID, "request": o.output}, nil)
}
