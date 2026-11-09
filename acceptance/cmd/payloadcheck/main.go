package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/protocol"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 0 && args[0] == "masque-netem" {
		return checkNetem(args[1:])
	}
	if len(args) < 3 || len(args) > 5 {
		return fmt.Errorf("use just payload-report DIRECTORY HOPS SINCE_UNIX_SECONDS [PLATFORM] [NODE_ADDRESSES]")
	}
	platform := ""
	if len(args) >= 4 {
		platform = args[3]
	}
	var addresses []string
	if len(args) == 5 && args[4] != "" {
		addresses = strings.Split(args[4], ",")
	}
	since, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil || since < 0 {
		return fmt.Errorf("invalid start timestamp")
	}
	var hops []int
	for _, value := range strings.Split(args[1], ",") {
		count, err := strconv.Atoi(value)
		if err != nil || (count != 1 && count != 3) {
			return fmt.Errorf("expected hops 1 and/or 3")
		}
		hops = append(hops, count)
	}
	var profiles []string
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			profiles = append(profiles, spec.ID)
		}
	}
	return evidence.CheckReports(args[0], hops, profiles, time.Unix(since, 0), platform, addresses...)
}
