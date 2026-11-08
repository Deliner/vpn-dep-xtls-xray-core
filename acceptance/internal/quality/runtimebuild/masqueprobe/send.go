package masqueprobe

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"

	"vpn-script/internal/platform/safeio"
)

const sendQueueSource = "cdb2905c1442662607ac4883288eb79ac66bac967f46e1121e3e0e6d02cdc70e"

func instrumentVerifiedSend(path string) ([]byte, error) {
	source, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != sendQueueSource {
		return nil, fmt.Errorf("unreviewed QUIC send queue source; diagnostic overlay refused")
	}
	return instrumentSendQueue(source)
}

func instrumentSendQueue(source []byte) ([]byte, error) {
	branch := []byte("\t\t\t\tif !isSendMsgSizeErr(err) && !errors.As(err, &tooLarge) {\n\t\t\t\t\treturn err\n\t\t\t\t}")
	if bytes.Count(source, branch) != 1 || bytes.Count(source, []byte("\t\"errors\"\n")) != 1 || bytes.Contains(source, []byte("NHOP_QUIC_SEND_SIZE_DROP")) {
		return nil, fmt.Errorf("ambiguous QUIC send-size instrumentation context")
	}
	observation := "\n\t\t\t\tfmt.Fprintf(os.Stderr, \"NHOP_QUIC_SEND_SIZE_DROP unix_ns=%d pid=%d bytes=%d gso=%d kernel=%t err=%q\\n\", time.Now().UnixNano(), os.Getpid(), len(e.buf.Data), e.gsoSize, isSendMsgSizeErr(err), err.Error())"
	modified := bytes.Replace(source, branch, append(bytes.Clone(branch), []byte(observation)...), 1)
	modified = bytes.Replace(modified, []byte("\t\"errors\"\n"), []byte("\t\"errors\"\n\t\"fmt\"\n\t\"os\"\n\t\"time\"\n"), 1)
	return format.Source(modified)
}
