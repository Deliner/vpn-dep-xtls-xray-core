package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

func TestCLIRegistrationPublicationAndRevocation(t *testing.T) {
	f := newAPIFixture(t)
	invite, token := enrollCLI(t, f)
	path := filepath.Join(f.root, "request.json")
	f.command("assignment", "--manifest", manifestFile(t, f.root), "--expected", "0", "--output", path)
	first := f.admin("publish", "--request", path)
	if retry := f.admin("publish", "--request", path); !bytes.Equal(first, retry) {
		t.Fatal("CLI retry changed the publication")
	}
	var nodes []wire.Node
	raw := f.admin("nodes")
	require(t, json.Unmarshal(raw, &nodes))
	if len(nodes) != 1 || nodes[0].Desired.Revision != 1 || !nodes[0].Enrolled || bytes.Contains(raw, []byte(token)) || bytes.Contains(raw, []byte(invite.Token)) {
		t.Fatal("CLI inventory lost state or exposed credentials")
	}
	assignment, err := f.client.Assignment(context.Background(), "node-a", token)
	require(t, err)
	if len(assignment) == 0 {
		t.Fatal("CLI publication was not delivered")
	}
	f.admin("revoke", "--node", "node-a")
	_, err = f.client.Assignment(context.Background(), "node-a", token)
	var response *fleetapi.HTTPError
	if !errors.As(err, &response) || response.Status != http.StatusForbidden {
		t.Fatalf("CLI revocation failed: %v", err)
	}
}

func enrollCLI(t *testing.T, f *apiFixture) (wire.Invitation, string) {
	return enrollCLINode(t, f, "node-a")
}

func enrollCLINode(t *testing.T, f *apiFixture, node string) (wire.Invitation, string) {
	t.Helper()
	path := filepath.Join(f.root, node+"-bootstrap.json")
	raw := f.admin("invite", "--node", node, "--output", path)
	var bootstrap pull.Bootstrap
	require(t, privateRead(path, &bootstrap))
	if bytes.Contains(raw, []byte(bootstrap.Invitation.Token)) {
		t.Fatal("CLI printed the enrollment token")
	}
	token, err := credential.NewToken()
	require(t, err)
	identity, err := f.client.Enroll(context.Background(), wire.Enrollment{NodeID: node, Token: bootstrap.Invitation.Token, Credential: token})
	require(t, err)
	if identity.FleetID != "test-fleet" || identity.NodeID != node {
		t.Fatal("CLI invitation belongs to another identity")
	}
	return bootstrap.Invitation, token
}

func TestCLIStagesCandidatesAndControlsTheSameDurableRollout(t *testing.T) {
	f := newAPIFixture(t)
	spec := wire.RolloutSpec{Version: 1, ID: "cli-rollout", ObserveSeconds: 10, FreshSeconds: 30, StageTimeoutSeconds: 90, ValiditySeconds: 3600}
	for _, node := range []string{"node-a", "node-b"} {
		enrollCLINode(t, f, node)
		var candidate wire.Candidate
		raw := f.admin("candidate", "--manifest", nodeManifestFile(t, f.root, node))
		require(t, json.Unmarshal(raw, &candidate))
		spec.Stages = append(spec.Stages, wire.RolloutStage{Name: node, Targets: []wire.RolloutTarget{{NodeID: node, CandidateSHA256: candidate.SHA256}}})
	}
	path := filepath.Join(f.root, "rollout.json")
	require(t, safeio.NewJSON(path, spec))
	created := f.admin("rollout-create", "--request", path)
	if retried := f.admin("rollout-create", "--request", path); !bytes.Equal(created, retried) {
		t.Fatal("CLI rollout retry changed intent")
	}
	var value wire.Rollout
	require(t, json.Unmarshal(f.admin("rollout", "--id", spec.ID), &value))
	if value.ID != spec.ID || value.Phase != "running" || value.TotalNodes != 2 || value.PublishedNodes != 0 {
		t.Fatal("CLI did not use the prepared rollout")
	}
	var summaries []wire.Rollout
	require(t, json.Unmarshal(f.admin("rollouts"), &summaries))
	if len(summaries) != 1 || len(summaries[0].Nodes) != 0 {
		t.Fatal("CLI did not return rollout summaries")
	}
	require(t, json.Unmarshal(f.admin("rollout-cancel", "--id", spec.ID), &value))
	if value.Phase != "canceled" || value.PublishedNodes != 0 {
		t.Fatal("CLI cancellation did not stop the prepared plan")
	}
}

func TestCLIPlansPrivateYAMLWithoutStartingTheRollout(t *testing.T) {
	f := newAPIFixture(t)
	for _, node := range []string{"node-a", "node-b"} {
		enrollCLINode(t, f, node)
		nodeManifestFile(t, f.root, node)
	}
	input, output := filepath.Join(f.root, "plan.yaml"), filepath.Join(f.root, "prepared.json")
	require(t, safeio.Create(input, []byte(cliRolloutDraft), 0600))
	f.admin("rollout-plan", "--input", input, "--output", output)
	var spec wire.RolloutSpec
	require(t, privateRead(output, &spec))
	require(t, spec.Validate())
	if spec.ID != "yaml-release" || spec.Stages[0].Targets[0].NodeID != "node-a" || spec.Stages[1].Targets[0].ExpectedRevision != 0 {
		t.Fatal("CLI preparation lost YAML order or registry snapshot")
	}
	var summaries []wire.Rollout
	require(t, json.Unmarshal(f.admin("rollouts"), &summaries))
	if len(summaries) != 0 {
		t.Fatal("planning started an automatic rollout before explicit creation")
	}
	f.admin("rollout-create", "--request", output)
	var value wire.Rollout
	require(t, json.Unmarshal(f.admin("rollout", "--id", spec.ID), &value))
	if value.TotalNodes != 2 || value.Phase != "running" {
		t.Fatal("prepared YAML request was not accepted by the real controller")
	}
}

const cliRolloutDraft = `version: 1
id: yaml-release
observe_seconds: 10
fresh_seconds: 30
stage_timeout_seconds: 90
validity_seconds: 3600
stages:
  - name: canary
    manifests: [node-a.json]
  - name: rest
    manifests: [node-b.json]
`
