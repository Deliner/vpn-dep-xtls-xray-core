package probe

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/sshutil"
)

type Dial func(context.Context, string, string) (net.Conn, error)

func reachable(ctx context.Context, address string, dial Dial) *bool {
	connection, err := dial(ctx, "tcp", address)
	if err == nil {
		connection.Close()
		value := true
		return &value
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return nil
	}
	var network net.Error
	if errors.Is(err, syscall.ECONNREFUSED) || (errors.As(err, &network) && network.Timeout()) {
		value := false
		return &value
	}
	return nil
}

func SSH(ctx context.Context, node model.DeployNode, run process.Runner, dial Dial) SSHResult {
	args := append(sshutil.Connect(node), "true")
	response, err := run(ctx, 30*time.Second, args...)
	result := SSHResult{Port: node.SSH.Port, Authenticated: err == nil && response.Success, Closed: map[string]*bool{}}
	result.Pass = result.Authenticated
	for _, number := range node.SSH.ClosedPorts {
		port := strconv.Itoa(number)
		value := reachable(ctx, net.JoinHostPort(node.Address, port), dial)
		result.Closed[port] = value
		result.Pass = result.Pass && value != nil && !*value
	}
	return result
}
