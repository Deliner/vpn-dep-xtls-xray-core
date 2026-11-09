package netem

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func phaseObservation(phase phase) error {
	if phase.Started.IsZero() || !phase.Finished.After(phase.Started) || phase.DurationMS != phase.Finished.Sub(phase.Started).Milliseconds() {
		return fmt.Errorf("invalid measurement window")
	}
	for _, link := range []link{phase.Before.Client, phase.Before.Exit, phase.After.Client, phase.After.Exit} {
		if err := linkObservation(link, phase.Phase); err != nil {
			return err
		}
	}
	if phase.Phase.Policy == "blocked" && phase.After.Client.Drops+phase.After.Exit.Drops <= phase.Before.Client.Drops+phase.Before.Exit.Drops {
		return fmt.Errorf("blackout has no observed encrypted packet drops")
	}
	beforeID, beforeDrops, err := originSocket(phase.Before.Exit.UDPSockets)
	if err != nil {
		return err
	}
	afterID, afterDrops, err := originSocket(phase.After.Exit.UDPSockets)
	if err != nil || beforeID != afterID || beforeDrops != afterDrops {
		return fmt.Errorf("origin socket changed or overflowed: %v", err)
	}
	return nil
}

func linkObservation(link link, state impairment) error {
	var queues []struct {
		Kind, Handle string
		Root         bool
		Drops        uint64
		Options      map[string]any
	}
	if err := json.Unmarshal(link.Queue, &queues); err != nil || len(queues) != 1 || queues[0].Kind != "netem" || queues[0].Drops != link.Drops ||
		queues[0].Handle != "10:" || !queues[0].Root || !reflect.DeepEqual(queues[0].Options, queueOptions(state.Name)) {
		return fmt.Errorf("invalid netem readback")
	}
	var interfaces []struct{ MTU int }
	if err := json.Unmarshal(link.Interface, &interfaces); err != nil || len(interfaces) != 1 || interfaces[0].MTU != state.MTU {
		return fmt.Errorf("invalid MTU readback")
	}
	return nil
}

func queueOptions(name string) map[string]any {
	options := map[string]any{"limit": float64(4096), "ecn": false, "gap": float64(0)}
	delay, jitter, loss := 0.0, 0.0, 0.0
	switch name {
	case "delay":
		delay, jitter = 0.02, 0.005
	case "loss":
		delay, loss = 0.01, 0.02
	case "mtu-1400", "mtu-1280":
		delay = 0.005
	case "blackout":
		loss = 1
	}
	if delay != 0 {
		options["delay"] = map[string]any{"delay": delay, "jitter": jitter, "correlation": float64(0)}
	}
	if loss != 0 {
		options["loss-random"] = map[string]any{"loss": loss, "correlation": float64(0)}
	}
	return options
}

func originSocket(raw string) (uint64, uint64, error) {
	var inode, drops uint64
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "020012C6:46A1" {
			continue
		}
		if len(fields) != 13 || inode != 0 {
			return 0, 0, fmt.Errorf("ambiguous origin socket record")
		}
		var err error
		inode, err = strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			return 0, 0, fmt.Errorf("missing origin inode")
		}
		drops, err = strconv.ParseUint(fields[12], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}
	if inode == 0 {
		return 0, 0, fmt.Errorf("missing origin socket")
	}
	return inode, drops, nil
}
