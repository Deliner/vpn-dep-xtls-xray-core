# Check concurrent HTTPS enrollment, signed assignments, reports and persistence using synthetic Fleet nodes only.
[group('fleet')]
fleet-scale-test nodes='1000' workers='8':
    @NHOP_FLEET_SCALE_NODES="$1" NHOP_FLEET_SCALE_WORKERS="$2" CGO_ENABLED=1 bash ops/quality/check.sh static-analysis go test -race -count=1 -v -timeout=10m ./internal/control/fleet/acceptance/scale -run '^TestFleetHTTPSScale$'

# Create a private Fleet signing authority and administrator credential; never overwrite existing state.
[group('fleet')]
fleet-init state fleet:
    go run ./cmd/nhop-fleet init --state "$1" --fleet "$2"

# Serve the Fleet HTTPS API using an existing private authority and TLS key; no host networking changes.
[group('fleet')]
fleet-serve state cert key listen='127.0.0.1:8443':
    go run ./cmd/nhop-fleet serve --state "$1" --cert "$2" --key "$3" --listen "$4"

# Register a pending node and write its short-lived invitation to a new private bootstrap file.
[group('fleet')]
fleet-invite endpoint credential node output ca='' group='main' validity='600':
    go run ./cmd/nhop-fleet invite --endpoint "$1" --credential "$2" --node "$3" --output "$4" --ca "$5" --group "$6" --validity "$7"

# Prepare a stable publication request ID and expected revision without contacting the controller.
[group('fleet')]
fleet-assignment manifest expected output validity='86400':
    go run ./cmd/nhop-fleet assignment --manifest "$1" --expected "$2" --output "$3" --validity "$4"

# Publish the prepared node assignment over HTTPS; retry the same request after a lost response.
[group('fleet')]
fleet-publish endpoint credential request ca='':
    go run ./cmd/nhop-fleet publish --endpoint "$1" --credential "$2" --request "$3" --ca "$4"

# Read node enrollment, desired revisions, latest reports and revocation state over HTTPS.
[group('fleet')]
fleet-nodes endpoint credential ca='':
    go run ./cmd/nhop-fleet nodes --endpoint "$1" --credential "$2" --ca "$3"

# Revoke a node's Fleet API access; this does not remove its already installed VPN.
[group('fleet')]
fleet-revoke endpoint credential node ca='':
    go run ./cmd/nhop-fleet revoke --endpoint "$1" --credential "$2" --node "$3" --ca "$4"

# Stage a validated node manifest over HTTPS; this does not change its desired assignment.
[group('fleet')]
fleet-candidate endpoint credential manifest ca='':
    go run ./cmd/nhop-fleet candidate --endpoint "$1" --credential "$2" --manifest "$3" --ca "$4"

# Validate all local YAML/manifests, snapshot current revisions and stage candidates into a reviewable request.
[group('fleet')]
fleet-rollout-plan endpoint credential input output ca='':
    go run ./cmd/nhop-fleet rollout-plan --endpoint "$1" --credential "$2" --input "$3" --output "$4" --ca "$5"

# Start the prepared ordered canary rollout; retry the same immutable request after a lost response.
[group('fleet')]
fleet-rollout-create endpoint credential request ca='':
    go run ./cmd/nhop-fleet rollout-create --endpoint "$1" --credential "$2" --request "$3" --ca "$4"

# List rollout summaries without private manifests or node credentials.
[group('fleet')]
fleet-rollouts endpoint credential ca='':
    go run ./cmd/nhop-fleet rollouts --endpoint "$1" --credential "$2" --ca "$3"

# Read the progress and health gate of every node in one rollout.
[group('fleet')]
fleet-rollout endpoint credential id ca='':
    go run ./cmd/nhop-fleet rollout --endpoint "$1" --credential "$2" --id "$3" --ca "$4"

# Stop future rollout publications; retain already published and installed configurations.
[group('fleet')]
fleet-rollout-cancel endpoint credential id ca='':
    go run ./cmd/nhop-fleet rollout-cancel --endpoint "$1" --credential "$2" --id "$3" --ca "$4"

# LINUX ROOT: initialize durable agent credentials and replay journal from a private bootstrap file.
[group('fleet')]
fleet-agent-init state bootstrap:
    go run ./cmd/nhop-fleet agent-init --state "$1" --bootstrap "$2"

# MUTATES LINUX NODE: continuously pull signed assignments and reconcile using the existing installer.
[group('fleet')]
fleet-agent-run state interval='30s':
    go run ./cmd/nhop-fleet agent-run --state "$1" --interval "$2"

# MUTATES LINUX NODE: perform one authenticated pull, installation and status report.
[group('fleet')]
fleet-agent-once state:
    go run ./cmd/nhop-fleet agent-run --state "$1" --once

# MUTATES LINUX FILES: snapshot this Fleet binary and install the verified inactive agent unit.
[group('fleet')]
fleet-agent-service-install state interval='30s':
    go run ./cmd/nhop-fleet agent-service-install --state "$1" --interval "$2"

# MUTATES LINUX NODE: enable and start the owned Fleet agent, allowing signed VPN installation.
[group('fleet')]
fleet-agent-service-start:
    go run ./cmd/nhop-fleet agent-service-start

# LINUX ROOT: stop and disable only the Fleet agent; retain the installed VPN and all journals.
[group('fleet')]
fleet-agent-service-stop:
    go run ./cmd/nhop-fleet agent-service-stop

# MUTATES LINUX FILES: unregister the stopped, disabled owned Fleet unit; retain credentials and versions.
[group('fleet')]
fleet-agent-service-remove:
    go run ./cmd/nhop-fleet agent-service-remove

# Apply signed configurations through the real Fleet agent/systemd only inside the owned Linux VM.
[group('fleet')]
fleet-agent-test:
    bash ops/testing/linux/run.sh fleet

# Preview read-only synthetic panel pages on 127.0.0.1:18086 for ten minutes; no real node or network changes.
[group('fleet')]
fleet-panel-preview:
    NHOP_PANEL_PREVIEW=1 go test -count=1 -v -timeout=11m -run '^TestPanelPreview$' ./internal/control/fleet/paneltest

# Prepare three owned Linux VMs and a private Lima user-v2 network; no host route or VPN changes.
[group('fleet')]
fleet-lab-setup: lab-setup
    bash ops/testing/linux/fleet/setup.sh

# Start the three owned Fleet VMs on their shared userspace network.
[group('fleet')]
fleet-lab-start:
    bash ops/testing/linux/fleet/vm.sh start

# Show all three Fleet lab machines.
[group('fleet')]
fleet-lab-status:
    bash ops/testing/linux/fleet/vm.sh status

# Stop the three Fleet lab machines and retain their disks and evidence.
[group('fleet')]
fleet-lab-stop:
    bash ops/testing/linux/fleet/vm.sh stop

# Execute a diagnostic command in fleet-a, fleet-b or fleet-c, never on the Mac host.
[group('fleet')]
fleet-lab-shell node +command:
    bash ops/testing/linux/fleet/vm.sh shell "$@"

# MUTATES THREE OWNED VMS: exercise real Fleet canary gates, recovery and three-hop TCP/UDP.
[group('fleet')]
fleet-canary-test:
    bash ops/testing/linux/fleet/canary.sh

# MUTATES THREE OWNED VMS: repeat exit-only TUIC updates with unchanged relays and bounded packet capture.
[group('fleet')]
fleet-tuic-recovery-test cycles='12':
    bash ops/testing/linux/fleet/canary.sh tuic-recovery "$1"
