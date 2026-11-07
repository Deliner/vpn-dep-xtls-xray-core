package routing

import "vpn-script/internal/platform/process"

const ownerProtocol = "191"

type rule struct {
	family, destination, table string
	priority                   int
}

type Manager struct {
	run           process.Runner
	rules         []rule
	interfaceName string
}
