package masqueprobe

import (
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

type sourceChange struct {
	Target    string
	SourceSHA string
	Modified  []byte
}

func inspectSources(root string) ([]sourceChange, string, error) {
	receivePath := filepath.Join(root, "http3", "state_tracking_stream.go")
	receive, err := safeio.Read(receivePath, 1024*1024)
	if err != nil {
		return nil, "", err
	}
	policy, receiveSHA, err := masqueQueuePolicy(receive)
	if err != nil {
		return nil, "", err
	}
	sendPath := filepath.Join(root, "send_queue.go")
	receive, err = instrumentMasqueQueue(receive)
	if err != nil {
		return nil, "", err
	}
	send, sendSHA, err := instrumentVerifiedSend(sendPath)
	if err != nil {
		return nil, "", err
	}
	controlPath := filepath.Join(root, "nhop_send_control_test.go")
	control, err := prepareSendControl(controlPath, sendSHA)
	if err != nil {
		return nil, "", err
	}
	return []sourceChange{
		{Target: receivePath, SourceSHA: receiveSHA, Modified: receive},
		{Target: sendPath, SourceSHA: sendSHA, Modified: send},
		{Target: controlPath, Modified: control},
	}, policy, nil
}
