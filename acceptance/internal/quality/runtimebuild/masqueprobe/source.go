package masqueprobe

import (
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
)

func sendQueuePolicy(changes []overlay.Change) string {
	for _, change := range changes {
		switch change.SourceSHA {
		case sendQueueAdaptiveSource:
			return "adaptive-pmtu"
		case sendQueueMTUSource:
			return "fatal-data-size"
		}
	}
	return "legacy-nonfatal-size"
}

func connectionQueuePolicy(changes []overlay.Change) string {
	for _, change := range changes {
		if change.SourceSHA == datagramBudgetSource {
			return "per-connection-bounded"
		}
	}
	return "fixed-128"
}

func inspectSources(root string) ([]overlay.Change, string, error) {
	receive, policy, err := inspectReceiveSources(root)
	if err != nil {
		return nil, "", err
	}
	send, err := inspectSendSources(root)
	if err != nil {
		return nil, "", err
	}
	return append(receive, send...), policy, nil
}

func inspectReceiveSources(root string) ([]overlay.Change, string, error) {
	receivePath := filepath.Join(root, "http3", "state_tracking_stream.go")
	receive, err := safeio.Read(receivePath, 1024*1024)
	if err != nil {
		return nil, "", err
	}
	policy, receiveSHA, err := masqueQueuePolicy(receive)
	if err != nil {
		return nil, "", err
	}
	receive, err = instrumentMasqueQueue(receive)
	if err != nil {
		return nil, "", err
	}
	datagramPath := filepath.Join(root, "datagram_queue.go")
	datagram, datagramSHA, err := instrumentVerifiedReceive(datagramPath)
	if err != nil {
		return nil, "", err
	}
	controlPath := filepath.Join(root, "nhop_receive_control_test.go")
	control, err := prepareReceiveControl(controlPath, datagramSHA)
	if err != nil {
		return nil, "", err
	}
	return []overlay.Change{
		{Target: receivePath, SourceSHA: receiveSHA, Modified: receive},
		{Target: datagramPath, SourceSHA: datagramSHA, Modified: datagram},
		{Target: controlPath, Modified: control},
	}, policy, nil
}

func inspectSendSources(root string) ([]overlay.Change, error) {
	sendPath := filepath.Join(root, "send_queue.go")
	send, sendSHA, err := instrumentVerifiedSend(sendPath)
	if err != nil {
		return nil, err
	}
	controlPath := filepath.Join(root, "nhop_send_control_test.go")
	control, err := prepareSendControl(controlPath, sendSHA)
	if err != nil {
		return nil, err
	}
	return []overlay.Change{
		{Target: sendPath, SourceSHA: sendSHA, Modified: send},
		{Target: controlPath, Modified: control},
	}, nil
}
