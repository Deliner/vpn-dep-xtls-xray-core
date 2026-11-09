package masqueprobe

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"

	"vpn-script/internal/platform/safeio"
)

const masqueQueueSource = "fb9d3debf96571f2a468a9bbccdeb6112659dcde63f990e5d6a7236cfc44b1df"
const masqueBudgetQueueSource = "64579fb88ffe9a2fd6d9c049f30f10f284a03869fa3e912acda1e6eb8a931de3"
const datagramQueueSource = "e4e7f7d508b99a8e9fcb3db1c81b41736e7db5c109c1dbe9ec2e18d52a92a48e"
const datagramBudgetSource = "495fcc6873be30dea2d6285a00826cd8c00eefbf173b06063dd5a0c572528800"

func instrumentVerifiedReceive(path string) ([]byte, string, error) {
	source, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(source))
	var modified []byte
	switch digest {
	case datagramQueueSource:
		modified, err = instrumentDatagramQueue(source)
	case datagramBudgetSource:
		modified, err = instrumentBudgetDatagramQueue(source)
	default:
		return nil, digest, fmt.Errorf("unreviewed QUIC receive queue source; diagnostic overlay refused")
	}
	return modified, digest, err
}

func instrumentDatagramQueue(source []byte) ([]byte, error) {
	snapshot := "\tnhopQueued := len(h.rcvQueue)\n"
	observation := "\tif !queued {\n" +
		"\t\tfmt.Fprintf(os.Stderr, \"NHOP_QUIC_QUEUE_DROP unix_ns=%d pid=%d queue=%p queued=%d limit=%d bytes=%d prefix=%x\\n\", time.Now().UnixNano(), os.Getpid(), h, nhopQueued, maxDatagramRcvQueueLen, len(data), data[:min(96, len(data))])\n\t}\n"
	return instrumentReceiveDrop(source, snapshot, observation)
}

func instrumentBudgetDatagramQueue(source []byte) ([]byte, error) {
	snapshot := "\tnhopQueued := h.rcvQueue.Len()\n\tnhopBuffered := h.rcvBytes\n"
	// The rejected frame stays owned by HandleDatagramFrame's caller until this
	// synchronous observation returns; accepted queue copies are never inspected.
	observation := "\tif !queued {\n" +
		"\t\tfmt.Fprintf(os.Stderr, \"NHOP_QUIC_QUEUE_DROP unix_ns=%d pid=%d queue=%p buffered=%d budget=%d queued=%d limit=%d bytes=%d prefix=%x\\n\", time.Now().UnixNano(), os.Getpid(), h, nhopBuffered, h.rcvMaxBytes, nhopQueued, h.rcvMaxPackets, len(f.Data), f.Data[:min(96, len(f.Data))])\n\t}\n"
	return instrumentReceiveDrop(source, snapshot, observation)
}

func instrumentReceiveDrop(source []byte, snapshot, observation string) ([]byte, error) {
	lock := []byte("\tvar queued bool\n\th.rcvMx.Lock()\n")
	branch := []byte("\tif !queued && h.logger.Debug() {")
	if bytes.Count(source, lock) != 1 || bytes.Count(source, branch) != 1 || bytes.Count(source, []byte("\t\"context\"\n")) != 1 || bytes.Contains(source, []byte("NHOP_QUIC_QUEUE_DROP")) {
		return nil, fmt.Errorf("ambiguous QUIC receive queue instrumentation context")
	}
	// Occupancy belongs to the existing receive lock; logging happens after it.
	modified := bytes.Replace(source, lock, append(bytes.Clone(lock), []byte(snapshot)...), 1)
	modified = bytes.Replace(modified, branch, append([]byte(observation), branch...), 1)
	modified = bytes.Replace(modified, []byte("\t\"context\"\n"), []byte("\t\"context\"\n\t\"fmt\"\n\t\"os\"\n\t\"time\"\n"), 1)
	return format.Source(modified)
}

func masqueQueuePolicy(source []byte) (string, string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(source))
	switch digest {
	case masqueQueueSource:
		return "fixed-32", digest, nil
	case masqueBudgetQueueSource:
		return "per-stream-bounded", digest, nil
	default:
		return "", digest, fmt.Errorf("unreviewed HTTP/3 queue source; diagnostic overlay refused")
	}
}

func instrumentMasqueQueue(source []byte) ([]byte, error) {
	condition, counter, err := queueDropBranch(source)
	if err != nil || bytes.Count(source, []byte("\t\"errors\"\n")) != 1 {
		return nil, fmt.Errorf("ambiguous HTTP/3 queue instrumentation context")
	}
	before := []byte("\tif " + condition + " {\n\t\treturn\n\t}")
	after := []byte("\tif " + condition + " {\n" +
		"\t\tfmt.Fprintf(os.Stderr, \"NHOP_H3_QUEUE_DROP unix_ns=%d pid=%d stream=%d queued=%d bytes=%d prefix=%x\\n\", time.Now().UnixNano(), os.Getpid(), s.StreamID(), " + counter + ", len(data), data[:min(64, len(data))])\n" +
		"\t\treturn\n\t}")
	value := bytes.Replace(source, before, after, 1)
	value = bytes.Replace(value, []byte("\t\"errors\"\n"), []byte("\t\"errors\"\n\t\"fmt\"\n\t\"time\"\n"), 1)
	return format.Source(value)
}

func queueDropBranch(source []byte) (condition, counter string, err error) {
	matched := 0
	for _, candidate := range [][2]string{
		{"len(s.queue) >= streamDatagramQueueLen", "len(s.queue)"},
		{"!s.queue.push(data)", "s.queue.count"},
	} {
		branch := []byte("\tif " + candidate[0] + " {\n\t\treturn\n\t}")
		count := bytes.Count(source, branch)
		if count != 0 {
			matched += count
			condition, counter = candidate[0], candidate[1]
		}
	}
	if matched != 1 {
		err = fmt.Errorf("expected exactly one reviewed queue admission branch")
	}
	return
}
