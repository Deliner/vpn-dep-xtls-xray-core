package sockets

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var pidPattern = regexp.MustCompile(`pid=(\d+)`)

// Parse accepts ss -H -O -lnptue output. Unknown or incomplete lines are errors,
// because silently dropping a listener could make a busy socket appear available.
func Parse(raw string) ([]Record, error) {
	var records []Record
	for index, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		record, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("invalid socket observation at line %d: %w", index+1, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func parseLine(line string) (Record, error) {
	fields := strings.Fields(line)
	if len(fields) < 6 || (fields[0] != "tcp" && fields[0] != "udp") {
		return Record{}, fmt.Errorf("expected a TCP/UDP listener")
	}
	if fields[0] == "tcp" && fields[1] != "LISTEN" || fields[0] == "udp" && fields[1] != "UNCONN" {
		return Record{}, fmt.Errorf("unexpected socket state")
	}
	record, err := binding(fields[4], fields[6:])
	if err != nil {
		return Record{}, err
	}
	record.Network = fields[0]
	for _, match := range pidPattern.FindAllStringSubmatch(line, -1) {
		if !slices.Contains(record.PIDs, match[1]) {
			record.PIDs = append(record.PIDs, match[1])
		}
	}
	return record, nil
}

func binding(endpoint string, details []string) (Record, error) {
	host, number, err := net.SplitHostPort(endpoint)
	if err != nil {
		return Record{}, fmt.Errorf("invalid local address/port")
	}
	port, err := strconv.Atoi(number)
	if err != nil || port < 1 || port > 65535 {
		return Record{}, fmt.Errorf("invalid local port")
	}
	wildcard := host == "*"
	if wildcard {
		host = "::"
	}
	host, device, _ := strings.Cut(host, "%")
	address, err := netip.ParseAddr(host)
	if err != nil {
		return Record{}, fmt.Errorf("local address must be numeric")
	}
	if address.Is6() && device != "" {
		address = address.WithZone(device)
	}
	dual, err := dualStack(address, details)
	if err == nil && wildcard && !dual {
		return Record{}, fmt.Errorf("ambiguous all-family wildcard")
	}
	return Record{Address: address.Unmap(), Interface: device, Port: port, DualStack: dual}, err
}

func dualStack(address netip.Addr, details []string) (bool, error) {
	address = address.WithZone("")
	value := ""
	for _, field := range details {
		if !strings.HasPrefix(field, "v6only:") {
			continue
		}
		if value != "" || (field != "v6only:0" && field != "v6only:1") {
			return false, fmt.Errorf("ambiguous IPv6 socket mode")
		}
		value = field
	}
	if address.Is6() && address.IsUnspecified() && value == "" {
		return false, fmt.Errorf("IPv6 wildcard requires observed v6only mode")
	}
	return address.Is6() && address.IsUnspecified() && value == "v6only:0", nil
}
