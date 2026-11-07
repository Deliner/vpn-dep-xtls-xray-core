package installer

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/sockets"
	"vpn-script/internal/platform/process"
)

func (i *Installer) sockets() ([]sockets.Record, error) {
	raw, err := i.command("ss", "-H", "-O", "-lnptue")
	if err != nil {
		return nil, err
	}
	if len(raw) >= process.OutputLimit {
		return nil, fmt.Errorf("socket observation reached the command output limit; completeness is unknown")
	}
	return sockets.Parse(string(raw))
}

func (i *Installer) mainPID() (string, error) {
	raw, err := i.command("systemctl", "show", "-p", "MainPID", "--value", service)
	return strings.TrimSpace(string(raw)), err
}

func (i *Installer) checkPorts(data model.NodeConfig) error {
	owner, _ := i.mainPID()
	bound, err := i.sockets()
	if err != nil {
		return err
	}
	for _, socket := range bound {
		for _, desired := range data.Listeners {
			if !sockets.Conflicts(socket, desired) {
				continue
			}
			if owner == "" || owner == "0" || len(socket.PIDs) != 1 || !ownedPID(socket.PIDs[0], owner) {
				return fmt.Errorf("socket %s/%s is owned by another service; nothing was stopped", net.JoinHostPort(socket.Address.String(), strconv.Itoa(socket.Port)), socket.Network)
			}
		}
	}
	return nil
}

func listenersReady(listeners []model.Listener, observed []sockets.Record, pid string) bool {
	if pid == "" || pid == "0" {
		return false
	}
	for _, listener := range listeners {
		found := false
		for _, socket := range observed {
			found = found || (sockets.Matches(socket, listener) && len(socket.PIDs) == 1 && ownedPID(socket.PIDs[0], pid))
		}
		if !found {
			return false
		}
	}
	return true
}

func (i *Installer) ready(data model.NodeConfig) error {
	var observationErr error
	for attempt := 0; attempt < 40; attempt++ {
		if i.Context.Err() != nil {
			return i.Context.Err()
		}
		pid, err := i.mainPID()
		sockets, socketErr := i.sockets()
		observationErr = socketErr
		if err == nil && socketErr == nil && listenersReady(data.Listeners, sockets, pid) && i.probe("systemctl", "is-active", "--quiet", service) {
			i.Sleep(500 * time.Millisecond)
			if i.probe("systemctl", "is-active", "--quiet", service) {
				return nil
			}
		}
		i.Sleep(250 * time.Millisecond)
	}
	if observationErr != nil {
		return fmt.Errorf("service failed process/listener checks: %w", observationErr)
	}
	return fmt.Errorf("service failed process/listener checks")
}

func ownedPID(pid, owner string) bool {
	for _, value := range []string{pid, owner} {
		if number, err := strconv.Atoi(value); err != nil || number <= 0 {
			return false
		}
	}
	if pid == owner {
		return true
	}
	status, err := os.ReadFile(filepath.Join("/proc", pid, "status"))
	if err != nil {
		return false
	}
	// PPid identifies the parent process even when another OS thread spawned the child.
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "PPid:" {
			return fields[1] == owner
		}
	}
	return false
}
