package canary

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func (f *fixture) capture(t *testing.T) {
	t.Helper()
	f.report.Captures = map[string]captureReceipt{}
	unit := "nhop-capture-" + filepath.Base(f.remote) + ".service"
	for _, node := range nodes {
		if f.property(t, node, unit, "LoadState") != "not-found" {
			t.Fatal("refusing an existing packet capture unit", node)
		}
		f.guest(t, node, "systemd-run", "--unit="+unit, "--collect", "--property=Type=exec", "--property=UMask=0077",
			"--property=RuntimeMaxSec=1800", "--property=LimitFSIZE=67108864", "--property=MemoryMax=64M",
			"/usr/bin/tcpdump", "-i", "any", "-s", "128", "-U", "-w", f.remote+"/tuic.pcap", "udp", "port", "24002")
		t.Cleanup(func() { f.finishCapture(t, node, unit) })
		if f.property(t, node, unit, "SubState") != "running" {
			t.Fatal("packet capture failed to start", node)
		}
	}
}

func (f *fixture) finishCapture(t *testing.T, node, unit string) {
	t.Helper()
	if f.property(t, node, unit, "SubState") != "running" {
		t.Error("packet capture ended before the scenario completed", node)
	}
	f.guest(t, node, "systemctl", "stop", unit)
	log := f.guest(t, node, "journalctl", "--no-pager", "-u", unit)
	require(t, safeio.Atomic(filepath.Join(f.root, node+"-capture.log"), log, 0600))
	if !strings.Contains(string(log), ": 0 packets dropped by kernel\n") {
		t.Error("capture has missing kernel drop evidence", node)
	}
	raw := f.saveRemote(t, node, f.remote+"/tuic.pcap", "tuic.pcap")
	digest := sha256.Sum256(raw)
	f.report.Captures[node] = captureReceipt{SHA256: hex.EncodeToString(digest[:]), Packets: packetCount(t, raw), Bytes: len(raw)}
}

func packetCount(t *testing.T, raw []byte) int {
	t.Helper()
	if len(raw) < 24 || !bytes.Equal(raw[:4], []byte{0xd4, 0xc3, 0xb2, 0xa1}) {
		t.Fatal("invalid binary pcap header")
	}
	if binary.LittleEndian.Uint32(raw[16:20]) != 128 || binary.LittleEndian.Uint32(raw[20:24]) != 276 {
		t.Fatal("packet capture must retain the declared Linux SLL2 snapshot")
	}
	offset, count := 24, 0
	for offset < len(raw) {
		if len(raw)-offset < 16 {
			t.Fatal("truncated pcap record")
		}
		size := int(binary.LittleEndian.Uint32(raw[offset+8 : offset+12]))
		original := binary.LittleEndian.Uint32(raw[offset+12 : offset+16])
		if size > 128 || uint32(size) > original || size > len(raw)-offset-16 {
			t.Fatal("invalid pcap record boundary")
		}
		offset += 16 + size
		count++
	}
	if count == 0 {
		t.Fatal("packet capture contains no TUIC packets")
	}
	return count
}
