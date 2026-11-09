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
	drops, err := queueObservation(value.Queue, historyOrdered(t))
	if err != nil {
		t.Fatalf("netem qdisc readback failed: %s / %v", value.Queue, err)
	}
	value.Drops = drops
	var links []struct {
		MTU int `json:"mtu"`
	}
	if err := json.Unmarshal(value.Link, &links); err != nil || len(links) != 1 || links[0].MTU != mtu {
		t.Fatalf("MTU readback failed: %s / %v", value.Link, err)
	}
	return value
}

func queueObservation(raw []byte, ordered bool) (uint64, error) {
	var queues []struct {
		Kind    string `json:"kind"`
		Handle  string `json:"handle"`
		Parent  string `json:"parent"`
		Root    bool   `json:"root"`
		Drops   uint64 `json:"drops"`
		Options struct {
			Limit int `json:"limit"`
		} `json:"options"`
	}
	want := 1
	if ordered {
		want++
	}
	if err := json.Unmarshal(raw, &queues); err != nil || len(queues) != want {
		return 0, fmt.Errorf("expected %d queues: %v", want, err)
	}
	var drops uint64
	seen := map[string]bool{}
	for _, queue := range queues {
		root := queue.Kind == "netem" && queue.Handle == "10:" && queue.Root && queue.Parent == ""
		child := ordered && queue.Kind == "pfifo" && queue.Handle == "20:" && !queue.Root && queue.Parent == "10:1"
		if (!root && !child) || seen[queue.Kind] || queue.Options.Limit != 4096 {
			return 0, fmt.Errorf("unexpected queue identity: %+v", queue)
		}
		seen[queue.Kind] = true
		drops += queue.Drops
	}
	return drops, nil
}

func TestQueueObservationRequiresSelectedTopology(t *testing.T) {
	root := `{"kind":"netem","handle":"10:","root":true,"drops":3,"options":{"limit":4096}}`
	child := `{"kind":"pfifo","handle":"20:","parent":"10:1","drops":5,"options":{"limit":4096}}`
	for _, raw := range []string{"[" + root + "," + child + "]", "[" + child + "," + root + "]"} {
		if drops, err := queueObservation([]byte(raw), true); err != nil || drops != 8 {
			t.Fatalf("lost queue identity or drops: %d / %v", drops, err)
		}
		if _, err := queueObservation([]byte(raw), false); err == nil {
			t.Fatal("accepted an unrequested child queue")
		}
	}
	for _, raw := range []string{"[" + root + "]", "[" + child + "," + child + "]",
		"[" + root + "," + strings.Replace(child, "10:1", "11:1", 1) + "]",
		"[" + root + "," + strings.Replace(child, "4096", "128", 1) + "]"} {
		if _, err := queueObservation([]byte(raw), true); err == nil {
			t.Fatal("accepted missing, duplicate or incorrect FIFO topology")
		}
	}
	if drops, err := queueObservation([]byte("["+root+"]"), false); err != nil || drops != 3 {
		t.Fatalf("ordinary netem observation changed: %d / %v", drops, err)
	}
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
