package packettrace

import (
	"fmt"
	"os"
	"time"
)

func nhopTraceTCP(direction string, connection any, packet []byte) {
	line, ok := nhopTCPObservation(direction, connection, packet)
	if ok {
		fmt.Fprintln(os.Stderr, line)
	}
}

func nhopTCPObservation(direction string, connection any, packet []byte) (string, bool) {
	header, ok := nhopTCPHeader(packet)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("NHOP_INNER_TCP unix_ns=%d pid=%d conn=%p phase=%s %s", time.Now().UnixNano(), os.Getpid(), connection, direction, header), true
}

func nhopTraceDevice(direction string, device, packet any, data []byte, queued int) {
	line, ok := nhopDeviceObservation(direction, device, packet, data, queued)
	if ok {
		fmt.Fprintln(os.Stderr, line)
	}
}

func nhopDeviceObservation(direction string, device, packet any, data []byte, queued int) (string, bool) {
	header, ok := nhopTCPHeader(data)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("NHOP_DEVICE_TCP unix_ns=%d pid=%d device=%p packet=%p phase=%s queued=%d %s", time.Now().UnixNano(), os.Getpid(), device, packet, direction, queued, header), true
}
