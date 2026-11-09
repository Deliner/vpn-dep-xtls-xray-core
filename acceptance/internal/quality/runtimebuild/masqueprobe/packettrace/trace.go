package packettrace

import (
	"fmt"
	"os"
	"time"
)

func nhopTracePacket(direction string, connection any, packet []byte) {
	line, ok := nhopPacketObservation(direction, connection, packet)
	if ok {
		fmt.Fprintln(os.Stderr, line)
	}
}

func nhopPacketObservation(direction string, connection any, packet []byte) (string, bool) {
	network, header := nhopPacketHeader(packet)
	if network == "" {
		return "", false
	}
	return fmt.Sprintf("NHOP_INNER_%s unix_ns=%d pid=%d conn=%p phase=%s %s", network, time.Now().UnixNano(), os.Getpid(), connection, direction, header), true
}

func nhopTraceDevice(direction string, device, packet any, data []byte, queued int) {
	line, ok := nhopDeviceObservation(direction, device, packet, data, queued)
	if ok {
		fmt.Fprintln(os.Stderr, line)
	}
}

func nhopDeviceObservation(direction string, device, packet any, data []byte, queued int) (string, bool) {
	network, header := nhopPacketHeader(data)
	if network == "" {
		return "", false
	}
	return fmt.Sprintf("NHOP_DEVICE_%s unix_ns=%d pid=%d device=%p packet=%p phase=%s queued=%d %s", network, time.Now().UnixNano(), os.Getpid(), device, packet, direction, queued, header), true
}

func nhopPacketHeader(packet []byte) (string, string) {
	if header, ok := nhopTCPHeader(packet); ok {
		return "TCP", header
	}
	if header, ok := nhopUDPHeader(packet); ok {
		return "UDP", header
	}
	return "", ""
}
