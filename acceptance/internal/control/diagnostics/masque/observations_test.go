package masque

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func observeNetwork(t *testing.T, mtu int) networkObservation {
	t.Helper()
	return networkObservation{Client: observeLink(t, "", mtu), Exit: observeLink(t, exitNamespace, mtu)}
}

func observeLink(t *testing.T, namespace string, mtu int) linkObservation {
	t.Helper()
	value := linkObservation{
		Queue:      json.RawMessage(shell(t, namespace, "tc", "-j", "-s", "qdisc", "show", "dev", "nhop-link")),
		Link:       json.RawMessage(shell(t, namespace, "ip", "-j", "-s", "link", "show", "dev", "nhop-link")),
		SNMP:       shell(t, namespace, "cat", "/proc/net/snmp"),
		UDPSockets: shell(t, namespace, "cat", "/proc/net/udp"),
	}
	var queues []struct {
		Kind  string `json:"kind"`
		Drops uint64 `json:"drops"`
	}
	if err := json.Unmarshal(value.Queue, &queues); err != nil || len(queues) != 1 || queues[0].Kind != "netem" {
		t.Fatalf("netem qdisc readback failed: %s / %v", value.Queue, err)
	}
	value.Drops = queues[0].Drops
	var links []struct {
		MTU int `json:"mtu"`
	}
	if err := json.Unmarshal(value.Link, &links); err != nil || len(links) != 1 || links[0].MTU != mtu {
		t.Fatalf("MTU readback failed: %s / %v", value.Link, err)
	}
	return value
}

func verifyBlackout(t *testing.T, report phaseReport) {
	t.Helper()
	if report.Phase.Policy != "blocked" {
		return
	}
	before := report.Before.Client.Drops + report.Before.Exit.Drops
	after := report.After.Client.Drops + report.After.Exit.Drops
	if after <= before {
		t.Errorf("blackout did not demonstrably drop encrypted traffic: %d -> %d", before, after)
	}
}

func verifyOriginReceive(t *testing.T, report phaseReport) {
	t.Helper()
	beforeID, before, err := originSocketDrops(report.Before.Exit.UDPSockets)
	if err != nil {
		t.Fatal(err)
	}
	afterID, after, err := originSocketDrops(report.After.Exit.UDPSockets)
	if err != nil {
		t.Fatal(err)
	}
	if beforeID != afterID || before != after {
		t.Errorf("echo origin changed or overflowed during %s: inode=%d/%d drops=%d/%d", report.Phase.Name, beforeID, afterID, before, after)
	}
}

func originSocketDrops(raw string) (uint64, uint64, error) {
	var inode, drops uint64
	address := fmt.Sprintf("020012C6:%04X", udpOriginPort) // /proc/net/udp encodes IPv4 in host byte order.
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != address {
			continue
		}
		if len(fields) != 13 || inode != 0 {
			return 0, 0, fmt.Errorf("ambiguous or incomplete origin socket record")
		}
		var err error
		inode, err = strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			return 0, 0, fmt.Errorf("invalid origin socket inode: %q", fields[9])
		}
		drops, err = strconv.ParseUint(fields[12], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}
	if inode == 0 {
		return 0, 0, fmt.Errorf("echo origin socket missing from evidence")
	}
	return inode, drops, nil
}

func TestOriginSocketEvidenceRejectsIncompleteOrAmbiguousRecords(t *testing.T) {
	line := "1067: 020012C6:46A1 00000000:0000 07 00000000:00000000 00:00000000 00000000 0 0 6976 2 pointer 77"
	inode, drops, err := originSocketDrops("sl local_address\n" + line)
	if err != nil || inode != 6976 || drops != 77 {
		t.Fatalf("socket loss identity was lost: %d %d %v", inode, drops, err)
	}
	for _, broken := range []string{"", line + "\n" + line, strings.Replace(line, " 77", "", 1), strings.Replace(line, "6976", "0", 1)} {
		if _, _, err := originSocketDrops(broken); err == nil {
			t.Fatal("accepted unreliable origin evidence")
		}
	}
}
