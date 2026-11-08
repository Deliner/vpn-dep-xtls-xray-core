// Package rollout coordinates durable, ordered Fleet updates. Registry remains
// responsible for signing and node CAS; journal owns private candidate/plan files.
package rollout

import (
	"sync"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout/journal"
	"vpn-script/internal/core/fleet/wire"
)

type Backend interface {
	Node(string) (wire.Node, error)
	Publish(string, wire.Publish) (wire.Desired, error)
}

type Manager struct {
	mu      sync.Mutex
	journal *journal.Store
	backend Backend
	now     func() time.Time
	closed  bool
}

func Open(directory string, backend Backend) (*Manager, error) {
	store, err := journal.Open(directory)
	if err != nil {
		return nil, err
	}
	return &Manager{journal: store, backend: backend, now: time.Now}, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	return m.journal.Close()
}

func (m *Manager) available() error {
	if m.closed {
		return registry.ErrClosed
	}
	return nil
}
