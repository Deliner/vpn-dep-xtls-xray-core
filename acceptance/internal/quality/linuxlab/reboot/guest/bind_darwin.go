package guest

import (
	"fmt"
	"syscall"
)

func bindInterface(_, _ string, _ syscall.RawConn) error {
	return fmt.Errorf("the NIC leak probe is Linux-only")
}
