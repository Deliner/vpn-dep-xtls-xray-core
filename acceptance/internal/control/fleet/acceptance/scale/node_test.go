package scale

import (
	"bytes"
	"context"
	"fmt"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func publication(node string) (wire.Publish, error) {
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "capacity", NodeID: node, Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	var err error
	data.Generation, err = safeio.Digest(data)
	if err != nil {
		return wire.Publish{}, err
	}
	raw, err := safeio.FormattedJSON(data)
	return wire.Publish{RequestID: contract.Digest([]byte("capacity-" + node)), ValiditySeconds: 3600, Manifest: raw}, err
}

func (f *fixture) exercise(ctx context.Context, node string) (wire.Node, error) {
	invite, err := f.client.Invite(ctx, f.admin, wire.Invite{NodeID: node, Group: "capacity", ValiditySeconds: 600})
	if err != nil {
		return wire.Node{}, err
	}
	token, err := credential.NewToken()
	if err != nil {
		return wire.Node{}, err
	}
	identity, err := f.client.Enroll(ctx, wire.Enrollment{NodeID: node, Token: invite.Token, Credential: token})
	if err != nil {
		return wire.Node{}, err
	}
	if identity.FleetID != fleetID || identity.NodeID != node {
		return wire.Node{}, fmt.Errorf("enrollment returned another identity")
	}
	f.tokens.Store(node, token)
	return f.assignment(ctx, invite, token)
}

func (f *fixture) assignment(ctx context.Context, invite wire.Invitation, token string) (wire.Node, error) {
	request, err := publication(invite.NodeID)
	if err != nil {
		return wire.Node{}, err
	}
	desired, err := f.client.Publish(ctx, invite.NodeID, f.admin, request)
	if err != nil {
		return wire.Node{}, err
	}
	raw, err := f.client.Assignment(ctx, invite.NodeID, token)
	if err != nil {
		return wire.Node{}, err
	}
	trust := contract.Trust{FleetID: fleetID, NodeID: invite.NodeID, PublicKey: invite.PublicKey}
	value, receipt, err := contract.Accept(raw, trust, contract.Receipt{}, time.Now())
	if err != nil {
		return wire.Node{}, err
	}
	if !bytes.Equal(value.Manifest, request.Manifest) || receipt.Revision != desired.Revision || contract.Digest(raw) != desired.EnvelopeSHA256 {
		return wire.Node{}, fmt.Errorf("signed assignment differs from publication")
	}
	return f.report(ctx, invite.NodeID, token, desired)
}

func (f *fixture) report(ctx context.Context, node, token string, desired wire.Desired) (wire.Node, error) {
	status := wire.Status{FleetID: fleetID, NodeID: node, Phase: "ready", Attempt: 1,
		AcceptedRevision: desired.Revision, AppliedRevision: desired.Revision, Generation: desired.Generation, AttemptedAt: time.Now().Unix()}
	if err := f.client.Report(ctx, token, status); err != nil {
		return wire.Node{}, err
	}
	value, err := f.client.Node(ctx, node, f.admin)
	if err != nil {
		return wire.Node{}, err
	}
	if !value.Enrolled || value.Desired != desired || value.Status == nil || *value.Status != status || value.LastSeen == 0 {
		return wire.Node{}, fmt.Errorf("reported state was lost or crossed node identities")
	}
	return value, nil
}
