package guest

import "syscall"

// A NIC-bound socket cannot be satisfied by the TUN route; delivery is a leak.
func bindInterface(_, _ string, raw syscall.RawConn) error {
	var bindError error
	err := raw.Control(func(fd uintptr) {
		bindError = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, "eth0")
	})
	if err != nil {
		return err
	}
	return bindError
}
