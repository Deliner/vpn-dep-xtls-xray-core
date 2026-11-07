package runtimebuild

import (
	"bytes"
	"fmt"
	"go/format"
)

func instrumentMasqueQueue(source []byte) ([]byte, error) {
	before := []byte("\tif len(s.queue) >= streamDatagramQueueLen {\n\t\treturn\n\t}")
	after := []byte("\tif len(s.queue) >= streamDatagramQueueLen {\n" +
		"\t\tfmt.Fprintf(os.Stderr, \"NHOP_H3_QUEUE_DROP unix_ns=%d pid=%d stream=%d queued=%d bytes=%d prefix=%x\\n\", time.Now().UnixNano(), os.Getpid(), s.StreamID(), len(s.queue), len(data), data[:min(64, len(data))])\n" +
		"\t\treturn\n\t}")
	if bytes.Count(source, before) != 1 || bytes.Count(source, []byte("\t\"errors\"\n")) != 1 {
		return nil, fmt.Errorf("ambiguous HTTP/3 queue instrumentation context")
	}
	value := bytes.Replace(source, before, after, 1)
	value = bytes.Replace(value, []byte("\t\"errors\"\n"), []byte("\t\"errors\"\n\t\"fmt\"\n\t\"time\"\n"), 1)
	return format.Source(value)
}
