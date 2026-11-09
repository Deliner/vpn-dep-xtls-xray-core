package packets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
)

const deviceSource = "7699af76176f60324873476fb62ff652604ed09182336a21762cfe61b308e2ba"
const fairDeviceSource = "9f19cf67815913851930cf29f4a32ef696232260b13909bdef33693912836ff2"

func InspectDevice(source string) ([]overlay.Change, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	root = filepath.Join(root, "proxy", "wireguard")
	path := filepath.Join(root, "netstack.go")
	original, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(original))
	if digest != deviceSource && digest != fairDeviceSource {
		return nil, fmt.Errorf("unreviewed netTun device source; diagnostic overlay refused")
	}
	fair := digest == fairDeviceSource
	modified, err := instrumentDevice(original, fair)
	if err != nil {
		return nil, err
	}
	helpers, err := preparePacketHelpers(root, "wireguard", deviceControl(fair))
	if err != nil {
		return nil, err
	}
	return append([]overlay.Change{{Target: path, SourceSHA: digest, Modified: modified}}, helpers...), nil
}

func instrumentDevice(source []byte, fair bool) ([]byte, error) {
	if bytes.Contains(source, []byte("nhopTraceDevice")) {
		return nil, fmt.Errorf("netTun source already instrumented")
	}
	depth := "len(tun.packets)"
	read := [2]string{"\tcase packet := <-tun.packets:\n", "\tcase packet := <-tun.packets:\n\t\tnhopTraceDevice(\"read\", tun, packet, packet.Bytes(), len(tun.packets))\n"}
	if fair {
		depth = "tun.packets.Len()"
		anchor := "\tsizes[0] = copy(buf[0][offset:], packet.Bytes())"
		read = [2]string{anchor, "\tnhopTraceDevice(\"read\", tun, packet, packet.Bytes(), " + depth + ")\n" + anchor}
	}
	contexts := [][2]string{
		read,
		{"\tpacket.Write(payload)\n", "\tpacket.Write(payload)\n\tnhopTraceDevice(\"offer\", tun, packet, packet.Bytes(), " + depth + ")\n"},
	}
	for _, context := range contexts {
		if bytes.Count(source, []byte(context[0])) != 1 {
			return nil, fmt.Errorf("ambiguous netTun packet instrumentation context")
		}
		source = bytes.Replace(source, []byte(context[0]), []byte(context[1]), 1)
	}
	return format.Source(source)
}
