package session

import (
	"os/exec"
	"sync"
)

type childExit struct {
	name string
	err  error
}

type children struct {
	commands []*exec.Cmd
	exits    chan childExit
	done     chan struct{}
	waiters  sync.WaitGroup
	once     sync.Once
	notice   sync.Once
}

func newChildren() *children {
	return &children{exits: make(chan childExit, 2), done: make(chan struct{})}
}
