package masqueprobe

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"

	"vpn-script/internal/platform/safeio"
)

const sendQueueSource = "cdb2905c1442662607ac4883288eb79ac66bac967f46e1121e3e0e6d02cdc70e"
const sendQueueMTUSource = "e16b2378b3e07f4342e0e11d151f77d224cb17bce08212f4a721acfa64e7903f"
const sendQueueAdaptiveSource = "166554d89ad2115429c7d6b41229318a150461c8591dbec9b284e20db72e75b0"

func instrumentVerifiedSend(path string) ([]byte, string, error) {
	source, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, "", err
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(source))
	var modified []byte
	switch sha {
	case sendQueueSource:
		modified, err = instrumentSendQueue(source)
	case sendQueueMTUSource:
		modified, err = instrumentMTUSendQueue(source)
	case sendQueueAdaptiveSource:
		modified, err = instrumentAdaptiveSendQueue(source)
	default:
		err = fmt.Errorf("unreviewed QUIC send queue source; diagnostic overlay refused")
	}
	return modified, sha, err
}

func instrumentAdaptiveSendQueue(source []byte) ([]byte, error) {
	branch := []byte("\tif entry.mtuProbe {\n\t\treturn nil\n\t}")
	recovery := []byte("\tif h.reportMTUError(entry) {\n\t\treturn nil\n\t}")
	if bytes.Count(source, branch) != 1 || bytes.Count(source, recovery) != 1 || bytes.Count(source, []byte("\t\"fmt\"\n")) != 1 || bytes.Contains(source, []byte("NHOP_QUIC_SEND_SIZE_DROP")) {
		return nil, fmt.Errorf("ambiguous adaptive QUIC MTU instrumentation context")
	}
	modified := bytes.Replace(source, branch, append([]byte(adaptiveSendObservation), branch...), 1)
	modified = bytes.Replace(modified, []byte("\t\"fmt\"\n"), []byte("\t\"fmt\"\n\t\"os\"\n\t\"time\"\n"), 1)
	return format.Source(modified)
}

// Observe the logical UDP size without consuming the recovery notification or
// touching connection state. A short final GSO segment may be below gsoSize.
const adaptiveSendObservation = `	size := entry.buf.Len()
	if entry.gsoSize != 0 {
		size = min(size, protocol.ByteCount(entry.gsoSize))
	}
	terminal := !entry.mtuProbe && (size <= protocol.MinInitialPacketSize || h.mtuErrors == nil)
	fmt.Fprintf(os.Stderr, "NHOP_QUIC_SEND_SIZE_DROP unix_ns=%d pid=%d bytes=%d gso=%d kernel=%t probe=%t terminal=%t failed_size=%d err=%q\n", time.Now().UnixNano(), os.Getpid(), len(entry.buf.Data), entry.gsoSize, isSendMsgSizeErr(err), entry.mtuProbe, terminal, size, err.Error())
`

func instrumentMTUSendQueue(source []byte) ([]byte, error) {
	branch := []byte("\tif entry.mtuProbe {\n\t\treturn nil\n\t}")
	if bytes.Count(source, branch) != 1 || bytes.Count(source, []byte("\t\"fmt\"\n")) != 1 || bytes.Contains(source, []byte("NHOP_QUIC_SEND_SIZE_DROP")) {
		return nil, fmt.Errorf("ambiguous QUIC MTU error instrumentation context")
	}
	observation := "\tfmt.Fprintf(os.Stderr, \"NHOP_QUIC_SEND_SIZE_DROP unix_ns=%d pid=%d bytes=%d gso=%d kernel=%t probe=%t terminal=%t err=%q\\n\", time.Now().UnixNano(), os.Getpid(), len(entry.buf.Data), entry.gsoSize, isSendMsgSizeErr(err), entry.mtuProbe, !entry.mtuProbe, err.Error())\n"
	modified := bytes.Replace(source, branch, append([]byte(observation), branch...), 1)
	modified = bytes.Replace(modified, []byte("\t\"fmt\"\n"), []byte("\t\"fmt\"\n\t\"os\"\n\t\"time\"\n"), 1)
	return format.Source(modified)
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
