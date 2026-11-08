package runtimebuild

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"
)

const masqueQueueSource = "fb9d3debf96571f2a468a9bbccdeb6112659dcde63f990e5d6a7236cfc44b1df"
const masqueBudgetQueueSource = "64579fb88ffe9a2fd6d9c049f30f10f284a03869fa3e912acda1e6eb8a931de3"

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
