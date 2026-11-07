package evidence

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/platform/process"
)

// ListenerPorts keeps planned lab listeners outside the kernel's automatic
// source-port ranges and below the matrix's internal bridges (starting at 20000).
// It observes configuration; it never changes host sysctls or reserves ports.
func ListenerPorts() ([]int, error) {
	raw, pairs, err := automaticPortRanges()
	if err != nil {
		return nil, fmt.Errorf("observe automatic port ranges: %w", err)
	}
	ranges, err := parsePortRanges(raw, pairs)
	if err != nil {
		return nil, err
	}
	return listenerCandidates(ranges)
}

func automaticPortRanges() (string, int, error) {
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
		return string(raw), 1, err
	}
	if runtime.GOOS == "darwin" {
		result, err := process.Run(context.Background(), 3*time.Second, "/usr/sbin/sysctl", "-n",
			"net.inet.ip.portrange.first", "net.inet.ip.portrange.last",
			"net.inet.ip.portrange.hifirst", "net.inet.ip.portrange.hilast")
		return string(result.Output), 2, err
	}
	return "", 0, fmt.Errorf("unsupported payload platform %s", runtime.GOOS)
}

func parsePortRanges(raw string, pairs int) ([][2]int, error) {
	fields := strings.Fields(raw)
	if pairs < 1 || len(fields) != 2*pairs {
		return nil, fmt.Errorf("missing or ambiguous automatic port ranges")
	}
	var ranges [][2]int
	for index := range pairs {
		first, firstErr := strconv.Atoi(fields[2*index])
		last, lastErr := strconv.Atoi(fields[2*index+1])
		if firstErr != nil || lastErr != nil || first < 1 || last < 1 || first > 65535 || last > 65535 {
			return nil, fmt.Errorf("invalid automatic port range")
		}
		// Darwin permits ascending and descending allocation ranges.
		ranges = append(ranges, [2]int{min(first, last), max(first, last)})
	}
	return ranges, nil
}

func listenerCandidates(ranges [][2]int) ([]int, error) {
	var ports []int
	for port := 10000; port < 20000; port++ {
		automatic := false
		for _, bounds := range ranges {
			automatic = automatic || port >= bounds[0] && port <= bounds[1]
		}
		if !automatic {
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("lab listener pool 10000–19999 overlaps automatic source ports; use an isolated test host with a disjoint range")
	}
	return ports, nil
}
