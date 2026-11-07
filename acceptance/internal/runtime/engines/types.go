package engines

import "vpn-script/internal/core/model"

// Compiler borrows a validated topology for one rendering operation. Do not
// mutate the topology while using it; construct a fresh compiler after edits.
type Compiler struct {
	topology *model.Topology
	services map[string][]string
	onlyXray bool
}

type graph struct {
	inbounds  map[string][]model.Object
	outbounds map[string]map[string]model.Object
	rules     map[string][]model.Object
	listeners []model.Listener
	used      map[int]bool
	nextPort  int
	udpExit   string
}
